package starts

import (
	"encoding/json"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/utils"
	"github.com/nyaruka/mailroom"
	_ "github.com/nyaruka/mailroom/hooks"
	"github.com/nyaruka/mailroom/models"
	"github.com/nyaruka/mailroom/queue"
	"github.com/nyaruka/mailroom/runner"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/stretchr/testify/assert"
)

func insertStart(db *sqlx.DB, uuid utils.UUID, restartParticipants bool, includeActive bool) {
	// note we don't bother with the many to many for contacts and groups in our testing
	db.MustExec(
		`INSERT INTO flows_flowstart(is_active, created_on, modified_on, uuid, restart_participants, include_active, 
									 contact_count, status, created_by_id, flow_id, modified_by_id)
							VALUES(TRUE, now(), now(), $1, $2, $3, 0, 'S', 1, $4, 1)`, uuid, restartParticipants, includeActive, models.SingleMessageFlowID)
}

func TestStarts(t *testing.T) {
	testsuite.Reset()
	ctx := testsuite.CTX()
	rp := testsuite.RP()
	db := testsuite.DB()
	rc := testsuite.RC()
	defer rc.Close()

	// insert a flow run for one of our contacts
	// TODO: can be replaced with a normal flow start of another flow once we support flows with waits
	db.MustExec(
		`INSERT INTO flows_flowrun(uuid, is_active, created_on, modified_on, responded, contact_id, flow_id, org_id)
		                    VALUES($1, TRUE, now(), now(), FALSE, $2, $3, 1);`, utils.NewUUID(), models.GeorgeID, models.SingleMessageFlowID)

	tcs := []struct {
		FlowID              models.FlowID
		GroupIDs            []models.GroupID
		ContactIDs          []flows.ContactID
		RestartParticipants bool
		IncludeActive       bool
		Queue               string
		ContactCount        int
		BatchCount          int
		TotalCount          int
	}{
		{models.SingleMessageFlowID, []models.GroupID{models.DoctorsGroupID}, nil, false, false, mailroom.BatchQueue, 121, 2, 121},
		{models.SingleMessageFlowID, []models.GroupID{models.DoctorsGroupID}, []flows.ContactID{models.CathyID}, false, false, mailroom.BatchQueue, 121, 2, 0},
		{models.SingleMessageFlowID, nil, []flows.ContactID{models.CathyID}, true, true, mailroom.HandlerQueue, 1, 1, 1},
		{models.SingleMessageFlowID, []models.GroupID{models.DoctorsGroupID}, []flows.ContactID{models.BobID}, false, false, mailroom.BatchQueue, 122, 2, 1},
		{models.SingleMessageFlowID, nil, []flows.ContactID{models.BobID}, false, false, mailroom.HandlerQueue, 1, 1, 0},
		{models.SingleMessageFlowID, nil, []flows.ContactID{models.BobID}, false, true, mailroom.HandlerQueue, 1, 1, 0},
		{models.SingleMessageFlowID, nil, []flows.ContactID{models.BobID}, true, true, mailroom.HandlerQueue, 1, 1, 1},
	}

	for i, tc := range tcs {
		startID := i + 1
		insertStart(db, utils.NewUUID(), false, false)

		// handle our start task
		start := models.NewFlowStart(
			models.NewStartID(startID), models.Org1, models.MessagingFlow, tc.FlowID,
			tc.GroupIDs, tc.ContactIDs, nil, false,
			tc.RestartParticipants, tc.IncludeActive,
			nil, nil,
		)
		err := CreateFlowBatches(ctx, db, rp, start)
		assert.NoError(t, err)

		// pop all our tasks and execute them
		var task *queue.Task
		count := 0
		for {
			task, err = queue.PopNextTask(rc, tc.Queue)
			assert.NoError(t, err)
			if task == nil {
				break
			}

			count++
			assert.Equal(t, mailroom.StartFlowBatchType, task.Type)
			batch := &models.FlowStartBatch{}
			err = json.Unmarshal(task.Task, batch)
			assert.NoError(t, err)

			_, err = runner.StartFlowBatch(ctx, db, rp, batch)
			assert.NoError(t, err)
		}

		// assert our count of batches
		assert.Equal(t, tc.BatchCount, count, "%d: unexpected batch count", i)

		// assert our count of total flow runs created
		testsuite.AssertQueryCount(t, db, `SELECT count(*) FROM flows_flowrun where flow_id = $1 AND start_id = $2 AND is_active = FALSE`,
			[]interface{}{tc.FlowID, startID}, tc.TotalCount, "%d: unexpected total run count", i)

		// flow start should be complete
		testsuite.AssertQueryCount(t, db, `SELECT count(*) FROM flows_flowstart where status = 'C' AND id = $1 AND contact_count = $2`,
			[]interface{}{startID, tc.ContactCount}, 1, "%d: start status not set to complete", i)
	}
}
