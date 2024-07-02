package graphqlbackend

import (
	"context"
	"fmt"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sourcegraph/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/processrestart"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/audit"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// canReloadSite is whether the current site can be reloaded via the API. Currently
// only goreman-managed sites can be reloaded. Callers must also check if the actor
// is an admin before actually reloading the site.
var isGoremanSite = processrestart.CanRestart()

// SRC_ENABLE_FRONTEND_SITE_RELOAD is env
var enableFrontendSiteReload = env.MustGetBool("SRC_ENABLE_FRONTEND_SITE_RELOAD", true, "Enables the /reload-site endpoint for reloading the site")

func (r *schemaResolver) ReloadSite(ctx context.Context) (*EmptyResponse, error) {
	// 🚨 SECURITY: Reloading the site is an disruptive action, so only admins
	// may do it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	if cloud.SiteConfig().SourcegraphOperatorAuthProviderEnabled() && enableFrontendSiteReload {
		// use k8s client to restart the frontend deployment rollout
		client, err := endpoint.LoadClient()
		if err != nil {
			return nil, err
		}
		ns := endpoint.Namespace(r.logger)
		data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s", "kubernetes.io/change-cause: "frontend restart requested"}}}}}`, time.Now().Format("20060102150405"))
		deploymentClient := client.AppsV1().Deployments(ns)

		currentUser, err := CurrentUser(ctx, r.db)
		if err != nil {
			return nil, err
		}
		audit.Log(ctx, r.logger, audit.Record{
			Entity: "GraphQL",
			Action: "frontend-reload",
			Fields: []log.Field{
				log.String("currentUser", currentUser.user.Username),
			},
		})
		r.logger.Info("Restarting k8s deployment")
		_, err = deploymentClient.Patch(ctx, "sourcegraph-frontend",
			k8stypes.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return nil, err
		}
		return &EmptyResponse{}, nil
	}

	if !isGoremanSite {
		return nil, errors.New("reloading site is not supported")
	}

	const delay = 750 * time.Millisecond
	log15.Warn("Will reload site (from API request)", "actor", actor.FromContext(ctx))
	time.AfterFunc(delay, func() {
		log15.Warn("Reloading site", "actor", actor.FromContext(ctx))
		if err := processrestart.Restart(); err != nil {
			log15.Error("Error reloading site", "err", err)
		}
	})

	return &EmptyResponse{}, nil
}
