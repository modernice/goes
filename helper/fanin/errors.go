package fanin

import "sync"

// Errors accepts multiple error channels and returns a single error channel.
// When the returned stop function is called or every input channel is closed,
// it stops and closes the returned error channel.
//
// When len(errs) == 0, Errors returns nil.
func Errors(errs ...<-chan error) (<-chan error, func()) {
	stop := make(chan struct{})
	var once sync.Once
	stopFn := func() { once.Do(func() { close(stop) }) }

	l := len(errs)
	if l == 0 {
		return nil, stopFn
	}

	if l == 1 {
		return errs[0], stopFn
	}

	out := make(chan error)
	var wg sync.WaitGroup
	wg.Add(l)
	for _, errs := range errs {
		go func(errs <-chan error) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				case err, ok := <-errs:
					if !ok {
						return
					}
					out <- err
				}
			}
		}(errs)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, stopFn
}
