package anthropic_test

import (
	"context"
	"testing"
	"time"

	// Packages
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_batch_001(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	batch, err := client.CreateBatch(context.TODO(), t.Name(), "claude-2", nil)
	require.NoError(err)
	require.NotNil(batch)
	t.Log("Batch:", batch)
}

func Test_batch_002(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	batch, err := client.CreateBatch(context.TODO(), t.Name(), "claude-2", nil)
	require.NoError(err)
	require.NotNil(batch)

	// return the batch
	batch2, err := client.GetBatch(context.TODO(), batch.Id)
	require.NoError(err)
	require.NotNil(batch2)
	require.Equal(batch.Id, batch2.Id)
	t.Log("Batch:", batch2)
}

func Test_batch_003(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	batch, err := client.CreateBatch(context.TODO(), t.Name(), "claude-2", nil)
	require.NoError(err)
	require.NotNil(batch)

	// cancel the batch
	batch2, err := client.CancelBatch(context.TODO(), batch.Id)
	require.NoError(err)
	require.NotNil(batch2)
	require.Equal(batch2.Status, "canceling")
	require.Equal(batch.Id, batch2.Id)

	t.Log("Batch:", batch2)
}

func Test_batch_004(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	batch, err := client.CreateBatch(context.TODO(), t.Name(), "claude-2", nil)
	require.NoError(err)
	require.NotNil(batch)

	// cancel the batch
	batch2, err := client.CancelBatch(context.TODO(), batch.Id)
	require.NoError(err)
	require.NotNil(batch2)
	require.Equal(batch2.Status, "canceling")
	require.Equal(batch.Id, batch2.Id)

	// Wait for the batch to be canceled
FOR_LOOP:
	for {
		select {
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for batch to be canceled")
		default:
			batch3, err := client.GetBatch(context.TODO(), batch.Id)
			require.NoError(err)
			require.NotNil(batch3)
			if batch3.Status == "ended" {
				t.Log("Batch canceled:", batch3)
				break FOR_LOOP
			}
			t.Log("Waiting for batch to be canceled, current status:", batch3.Status)
			time.Sleep(5 * time.Second)
		}
	}

	// delete the batch
	err = client.DeleteBatch(context.TODO(), batch.Id)
	require.NoError(err)
}

func Test_batch_005(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	// List batches without any options
	batches, err := client.ListBatches(context.TODO())
	require.NoError(err)
	require.NotNil(batches)

	t.Log("Batches:", batches)
}

func Test_batch_006(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	// List batches with limit
	batches, err := client.ListBatches(context.TODO(), anthropic.WithLimit(5))
	require.NoError(err)
	require.NotNil(batches)

	t.Log("Batches:", batches)
}

func Test_batch_007(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	// Create a batch first
	batch, err := client.CreateBatch(context.TODO(), t.Name(), "claude-2", nil)
	require.NoError(err)
	require.NotNil(batch)

	// List batches and verify our batch is in the list
	batches, err := client.ListBatches(context.TODO())
	require.NoError(err)
	require.NotNil(batches)
	require.NotEmpty(batches.Data)

	// Find our batch in the list
	found := false
	for _, b := range batches.Data {
		if b.Id == batch.Id {
			found = true
			break
		}
	}
	require.True(found, "Created batch should be in the list")

	// Cleanup - cancel and wait for end
	_, err = client.CancelBatch(context.TODO(), batch.Id)
	require.NoError(err)

	t.Log("Batches:", batches)
}

func Test_batch_008(t *testing.T) {
	require := require.New(t)
	require.NotNil(client)

	// List batches to find one that has ended
	batches, err := client.ListBatches(context.TODO())
	require.NoError(err)
	require.NotNil(batches)

	// Find an ended batch with results
	var endedBatchId string
	for _, b := range batches.Data {
		if b.Status == "ended" && b.ResultsUrl != nil {
			endedBatchId = b.Id
			break
		}
	}

	if endedBatchId == "" {
		t.Skip("No ended batch with results found")
	}

	// Get the batch results
	results, err := client.GetBatchResults(context.TODO(), endedBatchId)
	require.NoError(err)
	require.NotNil(results)

	t.Logf("Found %d results for batch %s", len(results), endedBatchId)
	for i, r := range results {
		t.Logf("Result %d: %s", i, r)
	}
}
