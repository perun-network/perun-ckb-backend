package client_test

import (
	"strconv"
	"testing"

	"github.com/nervosnetwork/ckb-sdk-go/v2/collector"
	"github.com/nervosnetwork/ckb-sdk-go/v2/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/client"
)

// Use the real SDK iterator: Next requires HasNext to load each new page.
type ckbIteratorPages struct {
	pages [][]*indexer.LiveCell
	calls int
}

func (p *ckbIteratorPages) GetCells(_ *indexer.SearchKey, _ indexer.SearchOrder, _ uint64, cursor string) (*indexer.LiveCells, error) {
	p.calls++
	page := 0
	if cursor != "" {
		var err error
		page, err = strconv.Atoi(cursor)
		if err != nil {
			return nil, err
		}
	}
	if page >= len(p.pages) {
		return &indexer.LiveCells{LastCursor: cursor}, nil
	}
	return &indexer.LiveCells{Objects: p.pages[page], LastCursor: strconv.Itoa(page + 1)}, nil
}

func TestCKBOnlyIterator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		pages [][]bool // true marks a token cell; false marks a CKB-only cell.
		want  []uint32
	}{
		{"token-only pages and trailing tokens", [][]bool{{true, true}, {true, true}, {false, true}, {false, true}}, []uint32{4, 6}},
		{"mixed page", [][]bool{{false, true, false}}, []uint32{0, 2}},
		{"only tokens", [][]bool{{true, true}, {true}}, nil},
		{"empty", nil, nil},
	} {
		for _, directNext := range []bool{false, true} {
			t.Run(tc.name+"/directNext="+strconv.FormatBool(directNext), func(t *testing.T) {
				source := &ckbIteratorPages{}
				var index uint32
				for _, page := range tc.pages {
					var cells []*indexer.LiveCell
					for _, token := range page {
						output := &types.CellOutput{Lock: &types.Script{HashType: types.HashTypeData}}
						if token {
							output.Type = &types.Script{HashType: types.HashTypeData1}
						}
						cells = append(cells, &indexer.LiveCell{OutPoint: &types.OutPoint{Index: index}, Output: output})
						index++
					}
					source.pages = append(source.pages, cells)
				}
				iter := client.NewCKBOnlyIterator(&collector.LiveCellIterator{LiveCellGetter: source, SearchKey: &indexer.SearchKey{}, Limit: 2})
				for _, want := range tc.want {
					if !directNext {
						require.True(t, iter.HasNext())
						calls := source.calls
						require.True(t, iter.HasNext(), "lookahead must not consume the next matching cell")
						require.Equal(t, calls, source.calls, "cached lookahead should not fetch another page")
					}
					cell := iter.Next()
					require.NotNil(t, cell)
					require.Nil(t, cell.Output.Type)
					require.Equal(t, want, cell.OutPoint.Index)
				}
				if !directNext {
					require.False(t, iter.HasNext(), "trailing token cells do not count as available CKB")
				}
				require.Nil(t, iter.Next(), "exhaustion must not panic")
				require.False(t, iter.HasNext())
				require.Nil(t, iter.Next())
			})
		}
	}
}
