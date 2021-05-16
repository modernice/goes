package taggingtest_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/aggregate/tagging/taggingtest"
	"github.com/modernice/goes/aggregate/tagging/taggingtest/mock_taggingtest"
	"github.com/modernice/goes/event"
)

func TestAggregate_nontagger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := newNontagger()

	testingT := mock_taggingtest.NewMockTestingT(ctrl)

	testingT.EXPECT().
		Fatalf("[tagging] %q Aggregate does not implement taggingtest.Tagger. Forgot to embed *tagging.Tagger?", "nontagger")

	taggingtest.Aggregate(testingT, a)
}

func TestAggregate_missingApply(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := newMissingApplyTagger()

	testingT := mock_taggingtest.NewMockTestingT(ctrl)

	testingT.EXPECT().
		Fatalf("[tagging] %q Aggregate does not apply tagging events. Forgot to implement `ApplyEvent`?", "tagger")

	taggingtest.Aggregate(testingT, a)
}

func TestAggregate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := newTagger()

	testingT := mock_taggingtest.NewMockTestingT(ctrl)

	taggingtest.Aggregate(testingT, a)
}

type nontagger struct {
	*aggregate.Base
}

type missingApplyTagger struct {
	*aggregate.Base
	*tagging.Tagger
}

type tagger struct {
	*aggregate.Base
	*tagging.Tagger
}

func newNontagger() *nontagger {
	return &nontagger{
		Base: aggregate.New("nontagger", uuid.New()),
	}
}

func newMissingApplyTagger() *missingApplyTagger {
	return &missingApplyTagger{
		Base:   aggregate.New("tagger", uuid.New()),
		Tagger: &tagging.Tagger{},
	}
}

func newTagger() *tagger {
	return &tagger{
		Base:   aggregate.New("tagger", uuid.New()),
		Tagger: &tagging.Tagger{},
	}
}

func (t *missingApplyTagger) ApplyEvent(event.Event) {}

func (t *tagger) ApplyEvent(evt event.Event) {
	t.Tagger.ApplyEvent(evt)
}
