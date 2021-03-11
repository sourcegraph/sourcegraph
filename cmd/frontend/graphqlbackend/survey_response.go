package graphqlbackend

import (
	"context"
	"errors"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot/hubspotutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/siteid"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type surveyResponseResolver struct {
	db             dbutil.DB
	surveyResponse *types.SurveyResponse
}

func (s *surveyResponseResolver) ID() graphql.ID {
	return marshalSurveyResponseID(s.surveyResponse.ID)
}
func marshalSurveyResponseID(id int32) graphql.ID { return relay.MarshalID("SurveyResponse", id) }

func (s *surveyResponseResolver) User(ctx context.Context) (*UserResolver, error) {
	if s.surveyResponse.UserID != nil {
		user, err := UserByIDInt32(ctx, s.db, *s.surveyResponse.UserID)
		if err != nil && errcode.IsNotFound(err) {
			// This can happen if the user has been deleted, see issue #4888 and #6454
			return nil, nil
		}
		return user, err
	}
	return nil, nil
}

func (s *surveyResponseResolver) Email() *string {
	return s.surveyResponse.Email
}

func (s *surveyResponseResolver) Score() int32 {
	return s.surveyResponse.Score
}

func (s *surveyResponseResolver) Reason() *string {
	return s.surveyResponse.Reason
}

func (s *surveyResponseResolver) Better() *string {
	return s.surveyResponse.Better
}

func (s *surveyResponseResolver) CreatedAt() DateTime {
	return DateTime{Time: s.surveyResponse.CreatedAt}
}

// SurveySubmissionInput contains a satisfaction (NPS) survey response.
type SurveySubmissionInput struct {
	// Emails is an optional, user-provided email address, if there is no
	// currently authenticated user. If there is, this value will not be used.
	Email *string
	// Score is the user's likelihood of recommending Sourcegraph to a friend, from 0-10.
	Score int32
	// Reason is the answer to "What is the most important reason for the score you gave".
	Reason *string
	// Better is the answer to "What can Sourcegraph do to provide a better product"
	Better *string
}

type surveySubmissionForHubSpot struct {
	Email           *string `url:"email"`
	Score           int32   `url:"nps_score"`
	Reason          *string `url:"nps_reason"`
	Better          *string `url:"nps_improvement"`
	IsAuthenticated bool    `url:"user_is_authenticated"`
	SiteID          string  `url:"site_id"`
}

// SubmitSurvey records a new satisfaction (NPS) survey response by the current user.
func (r *schemaResolver) SubmitSurvey(ctx context.Context, args *struct {
	Input *SurveySubmissionInput
}) (*EmptyResponse, error) {
	input := args.Input
	var uid *int32
	email := input.Email

	// If user is authenticated, use their uid and overwrite the optional email field.
	actor := actor.FromContext(ctx)
	if actor.IsAuthenticated() {
		uid = &actor.UID
		e, _, err := database.GlobalUserEmails.GetPrimaryEmail(ctx, actor.UID)
		if err != nil && !errcode.IsNotFound(err) {
			return nil, err
		}
		if e != "" {
			email = &e
		}
	}

	_, err := database.SurveyResponses(r.db).Create(ctx, uid, email, int(input.Score), input.Reason, input.Better)
	if err != nil {
		return nil, err
	}

	// Submit form to HubSpot
	if err := hubspotutil.Client().SubmitForm(hubspotutil.SurveyFormID, &surveySubmissionForHubSpot{
		Email:           email,
		Score:           args.Input.Score,
		Reason:          args.Input.Reason,
		Better:          args.Input.Better,
		IsAuthenticated: actor.IsAuthenticated(),
		SiteID:          siteid.Get(),
	}); err != nil {
		// Log an error, but don't return one if the only failure was in submitting survey results to HubSpot.
		log15.Error("Unable to submit survey results to Sourcegraph remote", "error", err)
	}

	return &EmptyResponse{}, nil
}

// FeedbackSubmissionInput contains a happiness feedback response.
type HappinessFeedbackSubmissionInput struct {
	// Score is the user's happiness rating, from 1-4.
	Score int32
	// Feedback is the answer to "What's going well? What could be better?".
	Feedback *string
	// The path that the happiness feedback was submitted from
	CurrentPath *string
}

type happinessFeedbackSubmissionForHubSpot struct {
	Email       *string `url:"email"`
	Score       int32   `url:"happiness_score"`
	Feedback    *string `url:"happiness_feedback"`
	CurrentPath *string `url:"happiness_current_url"`
	IsTest      bool    `url:"happiness_is_test"`
	SiteID      string  `url:"site_id"`
}

// SubmitHappinessFeedback records a new happiness feedback response by the current user.
func (r *schemaResolver) SubmitHappinessFeedback(ctx context.Context, args *struct {
	Input *HappinessFeedbackSubmissionInput
}) (*EmptyResponse, error) {
	var email *string

	if args.Input.Score < 1 || args.Input.Score > 4 {
		return nil, errors.New("Score must be a value between 1 and 4")
	}

	// If we are on Sourcegraph.com, we want to capture the email of the user
	if envvar.SourcegraphDotComMode() {
		actor := actor.FromContext(ctx)

		// If user is authenticated, use their uid and set the email field.
		if actor.IsAuthenticated() {
			e, _, err := database.UserEmails(r.db).GetPrimaryEmail(ctx, actor.UID)
			if err != nil && !errcode.IsNotFound(err) {
				return nil, err
			}
			if e != "" {
				email = &e
			}
		}
	}

	// Submit form to HubSpot
	if err := hubspotutil.Client().SubmitForm(hubspotutil.HappinessFeedbackFormID, &happinessFeedbackSubmissionForHubSpot{
		Email:       email,
		Score:       args.Input.Score,
		Feedback:    args.Input.Feedback,
		CurrentPath: args.Input.CurrentPath,
		IsTest:      env.InsecureDev,
		SiteID:      siteid.Get(),
	}); err != nil {
		// Log an error, but don't return one if the only failure was in submitting feedback results to HubSpot.
		log15.Error("Unable to submit happiness feedback results to Sourcegraph remote", "error", err)
	}

	return &EmptyResponse{}, nil
}
