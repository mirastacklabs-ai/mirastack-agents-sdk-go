package telemetrycache

import "sync"

func runWithConcurrency(taskCount int, limit int, fn func(i int)) {
	if taskCount <= 0 {
		return
	}
	if limit <= 0 || limit > taskCount {
		limit = taskCount
	}
	var wg sync.WaitGroup
	jobs := make(chan int, taskCount)
	for i := 0; i < taskCount; i++ {
		jobs <- i
	}
	close(jobs)

	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				fn(idx)
			}
		}()
	}
	wg.Wait()
}
