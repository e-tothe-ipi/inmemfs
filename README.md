# inmemfs

A simple in-memory filesystem built on go-fuse in Go.

Basically an exercise for me to learn Go and fuse.

To run:
	
	inmemfs [-debug] <mountpoint> &

To stop:

	umount <mountpoint>

### History

I originally tried using bazil.org/fuse but I encountered so many issues with memory. 

Seems like go-fuse is more mature so I'm trying that one for now.
