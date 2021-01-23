package stream_test

import (
	"context"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/cursor"
	"github.com/modernice/goes/aggregate/stream"
	ecursor "github.com/modernice/goes/event/cursor"
)

func BenchmarkStream1000Aggregates100Events(b *testing.B) { benchmarkStream(b, 1000, 100) }

func BenchmarkStream1000Aggregates200Events(b *testing.B) { benchmarkStream(b, 1000, 200) }

func BenchmarkStream1000Aggregates500Events(b *testing.B) { benchmarkStream(b, 1000, 500) }

func BenchmarkStream100Aggregates1000Events(b *testing.B) { benchmarkStream(b, 100, 1000) }

func BenchmarkStream200Aggregates1000Events(b *testing.B) { benchmarkStream(b, 200, 1000) }

func BenchmarkStream500Aggregates1000Events(b *testing.B) { benchmarkStream(b, 500, 1000) }

func benchmarkStream(b *testing.B, an, en int) {
	for n := 0; n < b.N; n++ {
		aggregates, _ := makeAggregates(an)
		aggregateMap := mapAggregates(aggregates)
		events, _ := makeEventsFor(aggregates, en)

		ecur := ecursor.New(events...)
		cur := stream.New(
			context.Background(),
			ecur,
			stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
				return aggregateMap[id]
			}),
		)

		_, err := cursor.All(context.Background(), cur)
		if err != nil {
			b.Fatalf("cursor.All should not fail: %v", err)
		}

		if err := cur.Close(context.Background()); err != nil {
			log.Println(err)
		}
	}
}
