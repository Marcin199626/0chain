package node

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"0chain.net/core/datastore"
	"github.com/stretchr/testify/require"
)

func TestRequestEntityHandlerNotModified(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))

	defer svr.Close()

	blockEntityMetadata := datastore.GetEntityMetadata("block")
	options := &SendOptions{Timeout: 3 * time.Second, MaxRelayLength: 0, CurrentRelayLength: 0, Compress: false}
	LatestFinalizedMagicBlockRequestor := RequestEntityHandler("/v1/block/get/latest_finalized_magic_block",
		options, blockEntityMetadata)

	var value int
	handler := func(ctx context.Context, entity datastore.Entity) (
		resp interface{}, err error) {
		value = 1
		return nil, nil
	}

	rhandler := LatestFinalizedMagicBlockRequestor(nil, handler)

	nd := Provider()
	nd.N2NHost = "127.0.0.1"
	ss := strings.Split(svr.URL, ":")
	var err error
	nd.Port, err = strconv.Atoi(ss[2])
	require.NoError(t, err)

	require.True(t, rhandler(context.Background(), nd))
	require.Equal(t, 0, value)
}

type customMockPool struct {
	mockPooler
	sendRequestsTo []*Node
}

func (cmp *customMockPool) sendRequestConcurrent(ctx context.Context, nds []*Node, handler SendHandler) *Node {
	cmp.sendRequestsTo = nds
	return nil
}

func TestRequestEntity(t *testing.T) {
	type expect struct {
		sendToNum int
		isPanic   bool
	}
	tt := []struct {
		name   string
		total  int
		opts   []Option
		expect *expect
	}{
		{
			name:  "100 nodes, default, to 10",
			total: 100,
			expect: &expect{
				sendToNum: 10,
			},
		},
		{
			name:  "50 nodes, default, to 5",
			total: 50,
			expect: &expect{
				sendToNum: 5,
			},
		},
		{
			name:  "45 nodes, default, to 5",
			total: 45,
			expect: &expect{
				sendToNum: 5,
			},
		},
		{
			name:  "44 nodes, default, to 4",
			total: 44,
			expect: &expect{
				sendToNum: 4,
			},
		},
		{
			name:  "4 nodes, default, to 4",
			total: 4,
			expect: &expect{
				sendToNum: 4,
			},
		},
		{
			name:  "3 nodes, default, to 3",
			total: 3,
			expect: &expect{
				sendToNum: 3,
			},
		},
		{
			name:  "0 nodes, default, to 0",
			total: 0,
			expect: &expect{
				sendToNum: 0,
			},
		},
		{
			name:  "100 nodes, percent 20, to 20",
			total: 100,
			opts: []Option{
				ToNodesPercent(20),
			},
			expect: &expect{
				sendToNum: 20,
			},
		},
		{
			name:  "100 nodes, percent 1, to 4",
			total: 100,
			opts: []Option{
				ToNodesPercent(1),
			},
			expect: &expect{
				sendToNum: 4,
			},
		},
		{
			name:  "10 nodes, min 5, to 5",
			total: 10,
			opts: []Option{
				ToNodesMin(5),
			},
			expect: &expect{
				sendToNum: 5,
			},
		},
	}

	handler := func(ctx context.Context, entity datastore.Entity) (
		resp interface{}, err error) {
		return nil, nil
	}

	requestor := func(urlParams *url.Values, handler datastore.JSONEntityReqResponderF) SendHandler {
		return nil
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			nds := prepareMockNodes(tc.total)

			np := &customMockPool{}
			np.On("GetNodesByLargeMessageTime").Return(nds)
			np.On("shuffleNodes", true).Return(nds)
			np.On("shuffleNodes", false).Return(nds)

			requestEntity(context.Background(), np, requestor, nil, handler, tc.opts...)

			require.Equal(t, tc.expect.sendToNum, len(np.sendRequestsTo))
		})
	}
}

func prepareMockNodes(n int) []*Node {
	nds := make([]*Node, n)
	for i := 0; i < n; i++ {
		nds[i] = &Node{
			Host: strconv.Itoa(i),
		}
	}
	return nds
}
