package importer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/dotcomdb"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/redislock"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/background"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/cloudsql"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type Importer struct {
	logger log.Logger

	dotcom *dotcomdb.Reader

	subscriptions *subscriptions.Store
	licenses      *subscriptions.LicensesStore

	tryAcquireFn func(context.Context) (acquired bool, release func(), _ error)
}

func NewHandler(
	ctx context.Context,
	logger log.Logger,
	dotcom *dotcomdb.Reader,
	enterprisePortal *database.DB,
	tryAcquireFn func(context.Context) (acquired bool, release func(), _ error),
) *Importer {
	return &Importer{
		logger:        logger,
		dotcom:        dotcom,
		subscriptions: enterprisePortal.Subscriptions(),
		licenses:      enterprisePortal.Subscriptions().Licenses(),
		tryAcquireFn:  tryAcquireFn,
	}
}

var _ goroutine.Handler = (*Importer)(nil)

// NewPeriodicImporter returns a periodic goroutine that runs an importer that
// reconciles subscriptions, licenses, and Cody Gateway access from dotcom into
// the Enterprise Portal database.
//
// If interval is 0, the importer is disabled.
func NewPeriodicImporter(
	ctx context.Context,
	logger log.Logger,
	dotcom *dotcomdb.Reader,
	enterprisePortal *database.DB,
	rs redispool.KeyValue,
	interval time.Duration,
) background.Routine {
	if interval == 0 {
		logger.Info("importer disabled")
		return background.NoopRoutine("dotcom.importer.disabled")
	}
	return goroutine.NewPeriodicGoroutine(
		ctx,
		NewHandler(ctx, logger, dotcom, enterprisePortal,
			func(ctx context.Context) (acquired bool, release func(), _ error) {
				if interval <= time.Second {
					return true, func() {}, nil
				}
				return redislock.TryAcquire(
					ctx,
					rs,
					// Use a different lock when the interval configuration is
					// changed significantly, to avoid being blocked by an old
					// configuration
					fmt.Sprintf("enterpriseportal.dotcomimporter.%d", int(interval.Seconds())),
					// Ensure lock is free by the time the next interval occurs
					interval-time.Second)
			}),
		goroutine.WithOperation(
			observation.NewContext(logger, observation.Tracer(trace.GetTracer())).
				Operation(observation.Op{
					Name: "dotcom.importer",
				})),
		goroutine.WithName("dotcom.importer"),
		goroutine.WithInterval(interval))
}

func (i *Importer) Handle(ctx context.Context) (err error) {
	if i.tryAcquireFn != nil {
		acquired, release, err := i.tryAcquireFn(ctx)
		if err != nil {
			return errors.Wrap(err, "acquire job")
		}
		trace.FromContext(ctx).
			SetAttributes(attribute.Bool("skipped", !acquired))
		if !acquired {
			trace.Logger(ctx, i.logger).Debug("skipping, job already acquired")
			return nil // nothing to do
		}
		defer func() {
			// Release on error for retry
			if err != nil {
				release()
			}
		}()
	}
	// Disable tracing on database interactions, because we could generate
	// traces with 10k+ spans in production.
	return i.ImportSubscriptions(cloudsql.WithoutTrace(ctx))
}

func (i *Importer) ImportSubscriptions(ctx context.Context) error {
	l := trace.Logger(ctx, i.logger)

	dotcomSubscriptions, err := i.dotcom.ListEnterpriseSubscriptions(ctx, dotcomdb.ListEnterpriseSubscriptionsOptions{})
	if err != nil {
		return err
	}
	l.Info("importing dotcom subscriptions",
		log.Int("subscriptions", len(dotcomSubscriptions)))

	wg := pool.New().
		WithErrors().
		WithMaxGoroutines(20)
	for _, dotcomSub := range dotcomSubscriptions {
		wg.Go(func() error {
			if err := i.importSubscription(ctx, dotcomSub); err != nil {
				return errors.Wrap(err, dotcomSub.ID)
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return err // routine handler will do error logging
	}
	l.Info("import succeeded")
	return nil
}

func (i *Importer) importSubscription(ctx context.Context, dotcomSub *dotcomdb.SubscriptionAttributes) (err error) {
	tr, ctx := trace.New(ctx, "importSubscription",
		attribute.String("dotcomSub.ID", dotcomSub.ID))
	defer tr.EndWithErr(&err)

	dotcomLicenses, err := i.dotcom.ListEnterpriseSubscriptionLicenses(ctx, []*subscriptionsv1.ListEnterpriseSubscriptionLicensesFilter{{
		Filter: &subscriptionsv1.ListEnterpriseSubscriptionLicensesFilter_SubscriptionId{
			SubscriptionId: dotcomSub.ID,
		},
	}}, 0) // list all licenses
	if err != nil {
		return errors.Wrap(err, "dotcom: list licenses")
	}

	tr.SetAttributes(attribute.Int("licenses", len(dotcomLicenses)))
	if len(dotcomLicenses) == 0 {
		// We don't care about this subscription, since it has no licenses -
		// skip over it during the migration
		tr.SetAttributes(attribute.Bool("no_licenses", true))
		return nil
	}

	// Construct the conditions we need to apply during the subscription
	// update
	var conditions []subscriptions.CreateSubscriptionConditionOptions
	if epSub, err := i.subscriptions.Get(ctx, dotcomSub.ID); err == nil {
		// No error - this subscription already exists.
		tr.SetAttributes(attribute.Bool("already_imported", true))
		if epSub.ArchivedAt == nil && dotcomSub.ArchivedAt != nil {
			// This subscription was archived post-import. We need to
			// add a new condition that reflects the archival event.
			tr.SetAttributes(attribute.Bool("archived", true))
			conditions = append(conditions,
				subscriptions.CreateSubscriptionConditionOptions{
					Status:         subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_ARCHIVED,
					TransitionTime: utctime.FromTime(*dotcomSub.ArchivedAt),
				})
		}
	} else if !errors.Is(err, subscriptions.ErrSubscriptionNotFound) {
		// Error occured, and it's not subscription-not-found - fail.
		return errors.Wrap(err, "check for already-imported subscription")
	} else {
		// Subscription doesn't exist, so we need to create all the
		// conditions that reflect the creation and archival events.
		tr.SetAttributes(attribute.Bool("already_imported", false))
		conditions = append(conditions,
			subscriptions.CreateSubscriptionConditionOptions{
				Status:         subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_CREATED,
				TransitionTime: utctime.FromTime(dotcomSub.CreatedAt),
			})
		if dotcomSub.ArchivedAt != nil {
			tr.SetAttributes(attribute.Bool("archived", true))
			conditions = append(conditions,
				subscriptions.CreateSubscriptionConditionOptions{
					Status:         subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_ARCHIVED,
					TransitionTime: utctime.FromTime(*dotcomSub.ArchivedAt),
				})
		}
		// Lastly, also create an initial-import event.
		conditions = append(conditions,
			subscriptions.CreateSubscriptionConditionOptions{
				Status:         subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_IMPORTED,
				TransitionTime: utctime.Now(),
				Message:        "Initial automated import to Enterprise Portal",
			})
	}

	// Apply updates to the subscription, creating it if it does not
	// exist yet.
	activeLicense := dotcomLicenses[0]
	if _, err := i.subscriptions.Upsert(ctx, dotcomSub.ID,
		subscriptions.UpsertSubscriptionOptions{
			DisplayName: func() *sql.NullString {
				for _, t := range activeLicense.Tags {
					parts := strings.SplitN(t, ":", 2)
					if len(parts) != 2 {
						continue
					}
					if parts[0] == "customer" && len(parts[1]) > 0 {
						return database.NewNullString(fmt.Sprintf("%s - %s",
							parts[1], dotcomSub.GenerateDisplayName()))
					}
				}
				return database.NewNullString(dotcomSub.GenerateDisplayName())
			}(),
			CreatedAt: utctime.FromTime(dotcomSub.CreatedAt),
			ArchivedAt: func() *utctime.Time {
				if dotcomSub.ArchivedAt == nil {
					return nil
				}
				return pointers.Ptr(utctime.FromTime(*dotcomSub.ArchivedAt))
			}(),
			SalesforceSubscriptionID: database.NewNullStringPtr(activeLicense.SalesforceSubscriptionID),
		},
		conditions...,
	); err != nil {
		return errors.Wrap(err, "upsert subscription")
	}

	// Import licenses belonging to this subscription
	for _, dotcomLicense := range dotcomLicenses {
		if err := i.importLicense(ctx, dotcomSub.ID, dotcomLicense); err != nil {
			return errors.Wrap(err, "import license")
		}
	}

	return nil
}

func nullInt64IfValid(v *int64) *sql.NullInt64 {
	if v == nil {
		return &sql.NullInt64{}
	}
	return database.NewNullInt64(*v)
}

func nullInt32IfValid(v *int32) *sql.NullInt32 {
	if v == nil {
		return &sql.NullInt32{}
	}
	return database.NewNullInt32(*v)
}

func (i *Importer) importLicense(ctx context.Context, subscriptionID string, dotcomLicense *dotcomdb.LicenseAttributes) (err error) {
	tr, ctx := trace.New(ctx, "importLicense",
		attribute.String("dotcomSub.ID", subscriptionID),
		attribute.String("dotcomLicense.ID", dotcomLicense.ID))
	defer tr.EndWithErr(&err)

	if epLicense, err := i.licenses.Get(ctx, dotcomLicense.ID); err == nil {
		// No error - this license already exists
		tr.SetAttributes(attribute.Bool("already_imported", true))
		if dotcomLicense.RevokedAt != nil && epLicense.RevokedAt == nil {
			// License was revoked post-import, we need to revoke it now on our
			// end
			tr.SetAttributes(attribute.Bool("revoked", true))
			if _, err := i.licenses.Revoke(ctx, dotcomLicense.ID, subscriptions.RevokeLicenseOpts{
				Message: pointers.DerefZero(dotcomLicense.RevokeReason),
				Time:    pointers.Ptr(utctime.FromTime(*dotcomLicense.RevokedAt)),
			}); err != nil {
				return errors.Wrap(err, "revoke license")
			}
		}
		return nil
	} else if !errors.Is(err, subscriptions.ErrSubscriptionLicenseNotFound) {
		// Error occured, and it's not license-not-found - fail.
		return errors.Wrap(err, "check for already-imported license")
	}

	tr.SetAttributes(attribute.Bool("already_imported", false))

	if _, err := i.licenses.CreateLicenseKey(ctx, subscriptionID,
		&subscriptions.DataLicenseKey{
			Info: license.Info{
				Tags:                     dotcomLicense.Tags,
				UserCount:                uint(pointers.DerefZero(dotcomLicense.UserCount)),
				CreatedAt:                dotcomLicense.CreatedAt,
				ExpiresAt:                pointers.DerefZero(dotcomLicense.ExpiresAt),
				SalesforceSubscriptionID: dotcomLicense.SalesforceSubscriptionID,
				SalesforceOpportunityID:  dotcomLicense.SalesforceOpportunityID,
			},
			SignedKey: dotcomLicense.LicenseKey,
		},
		subscriptions.CreateLicenseOpts{
			Time:       pointers.Ptr(utctime.FromTime(dotcomLicense.CreatedAt)),
			ExpireTime: utctime.FromTime(pointers.DerefZero(dotcomLicense.ExpiresAt)),
			// Use the existing license ID
			ImportLicenseID: dotcomLicense.ID,
		},
	); err != nil {
		return errors.Wrap(err, "create license")
	}

	var licenseImportErrors error
	if dotcomLicense.RevokedAt != nil {
		tr.SetAttributes(attribute.Bool("revoked", true))
		if _, err := i.licenses.Revoke(ctx, dotcomLicense.ID, subscriptions.RevokeLicenseOpts{
			Message: pointers.DerefZero(dotcomLicense.RevokeReason),
			Time:    pointers.Ptr(utctime.FromTime(*dotcomLicense.RevokedAt)),
		}); err != nil {
			licenseImportErrors = errors.Append(licenseImportErrors,
				errors.Wrapf(err, "import license %q", dotcomLicense.ID))
		}
	}
	if licenseImportErrors != nil {
		return licenseImportErrors
	}

	return nil
}
