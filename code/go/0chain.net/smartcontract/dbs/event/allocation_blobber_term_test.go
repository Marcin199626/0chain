package event

import (
	"0chain.net/chaincore/config"
	"0chain.net/chaincore/currency"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"strconv"
	"testing"
	"time"
)

func init() {
	logging.Logger = zap.NewNop()
}

func TestAllocationBlobberTerms(t *testing.T) {
	t.Skip("only for local debugging, requires local postgres")

	access := config.DbAccess{
		Enabled:         true,
		Name:            "events_db",
		User:            "zchain_user",
		Password:        "zchian",
		Host:            "localhost",
		Port:            "5432",
		MaxIdleConns:    100,
		MaxOpenConns:    200,
		ConnMaxLifetime: 20 * time.Second,
	}
	eventDb, err := NewEventDb(access)
	require.NoError(t, err)
	defer eventDb.Close()
	err = eventDb.Drop()
	require.NoError(t, err)
	err = eventDb.AutoMigrate()
	require.NoError(t, err)

	term1 := AllocationBlobberTerm{
		AllocationID:            encryption.Hash("mockAllocation_" + strconv.Itoa(0)),
		BlobberID:               encryption.Hash("mockBlobber_" + strconv.Itoa(0)),
		ReadPrice:               int64(currency.Coin(29)),
		WritePrice:              int64(currency.Coin(31)),
		MinLockDemand:           37.0,
		MaxOfferDuration:        39 * time.Minute,
		ChallengeCompletionTime: 41 * time.Minute,
	}

	term2 := AllocationBlobberTerm{
		AllocationID:            term1.AllocationID,
		BlobberID:               encryption.Hash("mockBlobber_" + strconv.Itoa(1)),
		ReadPrice:               int64(currency.Coin(41)),
		WritePrice:              int64(currency.Coin(43)),
		MinLockDemand:           47.0,
		MaxOfferDuration:        49 * time.Minute,
		ChallengeCompletionTime: 51 * time.Minute,
	}

	err = eventDb.addOrOverwriteAllocationBlobberTerm(term1)
	require.NoError(t, err, "Error while inserting Allocation's Blobber's AllocationBlobberTerm to event database")

	var term *AllocationBlobberTerm
	var terms []AllocationBlobberTerm
	terms, err = eventDb.GetAllocationBlobberTerms(term1.AllocationID, "")
	require.Equal(t, int64(1), len(terms), "AllocationBlobberTerm not getting inserted")

	err = eventDb.addOrOverwriteAllocationBlobberTerm(term2)
	require.NoError(t, err, "Error while inserting Allocation's Blobber's AllocationBlobberTerm to event database")

	terms, err = eventDb.GetAllocationBlobberTerms(term1.AllocationID, "")
	require.Equal(t, int64(2), len(terms), "Authorizer not getting inserted")

	_, err = eventDb.GetAllocationBlobberTerm(term1.AllocationID, term1.BlobberID)
	require.NoError(t, err, "Error while getting AllocationBlobberTerm from event Database")

	_, err = term2.exists(eventDb)
	require.NoError(t, err, "Error while checking if AllocationBlobberTerm exists in event Database")

	term2.MinLockDemand = 70.0
	err = eventDb.addOrOverwriteAllocationBlobberTerm(term2)
	require.NoError(t, err, "Error while inserting Allocation's Blobber's AllocationBlobberTerm to event database")

	term, err = eventDb.GetAllocationBlobberTerm(term2.AllocationID, term2.BlobberID)
	require.Equal(t, term2.MinLockDemand, term.MinLockDemand, "Error while overriding AllocationBlobberTerm in event Database")

	err = eventDb.Drop()
	require.NoError(t, err)
}
