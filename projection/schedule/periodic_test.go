package schedule_test

// func TestPeriodic_Subscribe(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	bus := chanbus.New()
// 	store := memstore.New()

// 	schedule := schedule.Periodically(store, 100*time.Millisecond, []string{"foo", "bar", "baz"})

// 	proj := projectiontest.NewMockProjection()
// 	applyErrors := make(chan error)
// 	appliedJobs := make(chan projection.Job)

// 	errs, err := schedule.Subscribe(ctx, func(job projection.Job) error {
// 		if err := job.Apply(job.Context(), proj); err != nil {
// 			applyErrors <- err
// 		}
// 		appliedJobs <- job
// 		return nil
// 	})
// 	if err != nil {
// 		t.Fatalf("Subscribe failed with %q", err)
// 	}

// 	events := []event.Event{
// 		event.New("foo", test.FooEventData{}),
// 		event.New("bar", test.FooEventData{}),
// 		event.New("baz", test.FooEventData{}),
// 		event.New("foobar", test.FooEventData{}),
// 	}

// 	if err := bus.Publish(ctx, events...); err != nil {
// 		t.Fatalf("publish Events: %v", err)
// 	}

// 	timer := time.NewTimer(3 * time.Second)
// 	defer timer.Stop()

// 	var applyCount int
// L:
// 	for {
// 		select {
// 		case <-timer.C:
// 			t.Fatalf("timed out. applyCount=%d/3", applyCount)
// 		case err := <-errs:
// 			t.Fatal(err)
// 		case err := <-applyErrors:
// 			t.Fatal(err)
// 		case <-appliedJobs:
// 			applyCount++
// 			if applyCount == 3 {
// 				select {
// 				case <-appliedJobs:
// 					t.Fatalf("only 3 Jobs should be created; got at least 4")
// 				case <-time.After(100 * time.Millisecond):
// 					break L
// 				}
// 			}
// 		}
// 	}

// 	proj.ExpectApplied(t, events[:3]...)
// }
