package flow

import (
	"context"
	"net/http"

	"github.com/nyaruka/goflow/envs"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/i18n"
	"github.com/nyaruka/goflow/utils"
	"github.com/nyaruka/mailroom/models"
	"github.com/nyaruka/mailroom/web"

	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func init() {
	web.RegisterRoute(http.MethodPost, "/mr/po/export", handleExport)
	web.RegisterJSONRoute(http.MethodPost, "/mr/po/import", handleImport)
}

// Exports a PO file from the given set of flows.
//
//   {
//     "org_id": 123,
//     "flow_ids": [123, 354, 456],
//     "language": "spa",
//     "exclude_arguments": true
//   }
//
type exportRequest struct {
	OrgID            models.OrgID    `json:"org_id"  validate:"required"`
	FlowIDs          []models.FlowID `json:"flow_ids" validate:"required"`
	Language         envs.Language   `json:"language" validate:"omitempty,language"`
	ExcludeArguments bool            `json:"exclude_arguments"`
}

func handleExport(ctx context.Context, s *web.Server, r *http.Request, rawW http.ResponseWriter) error {
	request := &exportRequest{}
	if err := utils.UnmarshalAndValidateWithLimit(r.Body, request, web.MaxRequestBytes); err != nil {
		return errors.Wrapf(err, "request failed validation")
	}

	flows, err := loadFlows(ctx, s.DB, request.OrgID, request.FlowIDs)
	if err != nil {
		return err
	}

	var excludeProperties []string
	if request.ExcludeArguments {
		excludeProperties = []string{"arguments"}
	}

	po, err := i18n.ExtractFromFlows("Generated by mailroom", request.Language, excludeProperties, flows...)
	if err != nil {
		return errors.Wrapf(err, "unable to extract PO from flows")
	}

	w := middleware.NewWrapResponseWriter(rawW, r.ProtoMajor)
	w.Header().Set("Content-type", "text/x-gettext-translation")
	w.WriteHeader(http.StatusOK)
	po.Write(w)
	return nil
}

// Imports translations from a PO file into the given set of flows.
//
//   {
//     "org_id": 123,
//     "flow_ids": [123, 354, 456],
//     "language": "spa"
//   }
//
type importForm struct {
	OrgID    models.OrgID    `form:"org_id"  validate:"required"`
	FlowIDs  []models.FlowID `form:"flow_ids" validate:"required"`
	Language envs.Language   `form:"language" validate:"required"`
}

func handleImport(ctx context.Context, s *web.Server, r *http.Request) (interface{}, int, error) {
	form := &importForm{}
	if err := web.DecodeAndValidateMultipartForm(form, r); err != nil {
		return err, http.StatusBadRequest, nil
	}

	poFile, _, err := r.FormFile("po")
	if err != nil {
		return errors.Wrapf(err, "missing po file on request"), http.StatusBadRequest, nil
	}

	po, err := i18n.ReadPO(poFile)
	if err != nil {
		return errors.Wrapf(err, "invalid po file"), http.StatusBadRequest, nil
	}

	flows, err := loadFlows(ctx, s.DB, form.OrgID, form.FlowIDs)
	if err != nil {
		return err, http.StatusBadRequest, nil
	}

	err = i18n.ImportIntoFlows(po, form.Language, flows...)
	if err != nil {
		return err, http.StatusBadRequest, nil
	}

	return flows, http.StatusOK, nil
}

func loadFlows(ctx context.Context, db *sqlx.DB, orgID models.OrgID, flowIDs []models.FlowID) ([]flows.Flow, error) {
	// grab our org
	org, err := models.GetOrgAssets(ctx, db, orgID)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load org assets")
	}

	flows := make([]flows.Flow, len(flowIDs))
	for i, flowID := range flowIDs {
		dbFlow, err := org.FlowByID(flowID)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to load flow with ID %d", flowID)
		}

		flow, err := org.SessionAssets().Flows().Get(dbFlow.UUID())
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read flow with UUID %s", string(dbFlow.UUID()))
		}

		flows[i] = flow
	}
	return flows, nil
}
