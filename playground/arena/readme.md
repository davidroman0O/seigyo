I've learned from the previous iteration of searching a way to make an actor-model and make it in a way to manage an application that 
- I need to try arena allocations
- I need to pre-allocate `Kind` and share a copy on each goroutine of the workers
  
I wish also to explore integrating `wasm` to allow plug-n-play backend

I hate the idea of having my hardware being under-used and i would appreciate to host multiple apps for one node

Let's explore the arena with `Kind` pre-allocation first
