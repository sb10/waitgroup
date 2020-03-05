# waitgroup
Like sync.WaitGroup, but with a timeout and debugging to show what you're
waiting for if a WaitGroup gets stuck waiting for too long or forever.

## Usage
```go
import "github.com/sb10/waitgroup"

wg := waitgroup.New()
loc := wg.Add(1)
go func() {
	defer wg.Done(loc)
	// ...
}()
loc2 := wg.Add(1)
go func() {
	// ... forgot to do wg.Done(loc2), or goroutine gets stuck and never defers
  // the Done() call
}()
wg.Wait(5 * time.Second) // tells you loc2 wasn't done
```
