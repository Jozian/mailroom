package web

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/engine"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/goflow/utils"
	"github.com/nyaruka/mailroom/models"
	"github.com/pkg/errors"
)

// Represents a surveyor submission
//
//   {
//     "session": {...},
//     "events": {...}
//   }
//
type surveyorSubmitRequest struct {
	Session json.RawMessage   `json:"session"`
	Events  []json.RawMessage `json:"events"`
}

type surveyorSubmitResponse struct {
}

// handles a surveyor request
func (s *Server) handleSurveyorSubmit(ctx context.Context, r *http.Request) (interface{}, int, error) {
	request := &surveyorSubmitRequest{}
	if err := utils.UnmarshalAndValidateWithLimit(r.Body, request, maxRequestBytes); err != nil {
		return nil, http.StatusBadRequest, errors.Wrapf(err, "request failed validation")
	}

	// grab our org
	orgID := ctx.Value(orgIDKey).(models.OrgID)
	org, err := models.NewOrgAssets(s.ctx, s.db, orgID, nil)
	if err != nil {
		return nil, http.StatusBadRequest, errors.Wrapf(err, "unable to load org assets")
	}

	// read our session
	assets, err := models.NewSessionAssets(org)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	session, err := engine.ReadSession(assets, engine.NewDefaultConfig(), httpClient, request.Session)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	// and our events
	sessionEvents := make([]flows.Event, 0, len(request.Events))
	for _, e := range request.Events {
		event, err := events.ReadEvent(e)
		if err != nil {
			return nil, http.StatusBadRequest, errors.Wrapf(err, "unable to unmarshal event: %s", string(e))
		}
		sessionEvents = append(sessionEvents, event)
	}

	// create / assign our contact
	urn := urns.NilURN
	if len(session.Contact().URNs()) > 0 {
		urn = session.Contact().URNs()[0].URN()
	}

	// create / fetch our contact based on the highest priority URN
	contactID, err := models.CreateContact(ctx, s.db, org, assets, urn)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrapf(err, "unable to look up contact")
	}

	// load that contact to get the current groups and UUID

	// set the UUID, ID and groups on the session contact

	// write everything out
	if session == nil {
		return nil, http.StatusBadRequest, errors.New("no session read")
	}

	// and our user id
	_, valid := ctx.Value(userIDKey).(int)
	if !valid {
		return nil, http.StatusInternalServerError, errors.Wrapf(err, "unable to request user")
	}

	return &surveyorSubmitResponse{}, http.StatusOK, nil
}
