# Gui Rava's tftpd assignment

I followed TFTP RFC's (https://tools.ietf.org/html/rfc1350) , but none of the amendements (e.g RFCs 1785, 2349 etc).

The application is in cmd/tftpd/main.go , and it uses the code packaged under pkg/tftpd.

When starting the application, it listens on 2 ports:
- UDP port 69    : the TFTP service
- TCP port 8069  : the admin REST interface

The admin REST interface was not part of the assignment but it makes testing & development much easier so it was worth the few extra lines of code. 

I have experience writing network applications and you will notice that the structure of this application is a bit overkill given the simplicity of the task. However, I can't count how many times "little projects" end up sticking around..

So I typically start any project with a few usual compartmentalizations. It's a big topic, but in short, I'd recommend a look at the 12factor.net website, it sums it up pretty well. In our case, I didn't create all the usual "compartments" but just a few :

1. the application (server.go)
2. the application configuration (config.go)
3. logging (logger.go)

Then filemanager.go handles files, socket.go is a few network utility wrappers, and I made minor modifications to the write.go you provided.

Please keep in mind this is my first project in Go !

I am still ramping up, especially when it comes to syntax choices. I go back and forth on silly things like
     if x,err:=foo();err!=nil ...
 or
 	 var x ..
	 if x,err=foo()..

Also, I am pretty sure that the socket.go file I wrote would look very different with more experience with the standard library: at this point I wrote my own wrapper functions to emulate library functions I am used to. 

## Main design

Although we are using UDP for transport, TFTP handles incoming requests on one port (UDP 69) and hands of request processing to
dynamically created sockets using ephemeral ports.

So effectively, to use TCP terminology:
- the incoming requests arrive on an accept loop
- the requet processing occurs on session sockets.

At a given time when the server is processing N requests, it runs N+2 threads:
1. the main thread is the accept loop, waiting for clients and sending them off to session threads
2. the REST admin interface serves on port 8069 in its own thread
3. N threads, 1 per session.

Each session thread creates a "UDP connection" with a tftp client

## Logging

Logging is done to the console and to 2 files: 
- tftpd.log
- tftpd_requests.log

At this point, these 2 files get truncated every time you start the application (see Implementation Notes below)


## The REST admin interface

This was a useful tool for developing and testing the app, and it could also end up as a feature. There are 3 endpoints:
- /  : returns a JSON object of the serialization of the application object. This is particularly useful to see what files are currently stored in memory.
- /shutdown : graceful shutdown of the application
- /clear : empty all files stored in memory

Note however, that it is a debug tool. If we actually wanted to use it in production, the admin interface code would need to be audited: in particular, calling the /clear endpoint clears out the file list without checking if anybody else is currently using it : it is meant to be used in a testing scenario where you know who's using your server.

## Implementation notes

- I moved all file handling to a separate unit : FileManager. The motivation was 2-fold: it makes the rest of the code easier to read, and it allows for design changes: for instance if we wanted to switch to a FS file TFTP service instead of the current all-memory storage, modifications would happen mostly in FileManager, and the server code would be left probably mostly intact.
- Request handling is described in the RFC as a lockstep process, and I ended up writing in server.go lockStepReceiveData() and lockStepSendData() but I'm pretty sure if I was to spend more time on this code, these 2 functions would coalesce into a single one with more parameters.
- logging is trivial, and does not handle rotation. I didn't want to spend more time on this because there must be good open-source packages to handle this well, it would be silly to write hand-made logging code beyond the simple solution I have right now: logging is often more complicated than it seems.


## Testing

I wrote a few unit tests but the emphasis is on the functional test script tests/tftp_functional_test.sh .
It is a bash script that needs curl and expect.
It calls the other script in the same directory: putputget.expect

There are 2 reasons why I focused on that 1 script instead of comprehensive unit tests:
1. I am new to Go and I focused my effort on learning to code with it ; 
   The test framework seems to be another significant task that I will 
   eventually tackle but it didn't seem efficient to work on that right now. 
   - For instance, when writing pkg/tftp/filemanager_test.go, I came up with a little
     snippet to generate big strings.. I also hand wrote a min() function ! 
   - In pkg/tftp/server_test.go , I started with a init/deinit sequence test, but then
     I realized I need to look at how to run a server (with listening sockets) in a test
     environment in Go.

2. the project is rather small and self-contained, and so focusing on testing blackbox 
   behavior is more relevant

This just shows I am not familiar with go test, but it's just part of the learning I need to do.


## Known issues

- 16bit block numbers are used during the lockstep exchange => if you send more than 65535 packets, they roll over => very unlikely, but imagine you send data packet #65535 to the client and it gets lost, the client will ask for it again, but now the server is at #0 . This may not be a problem (if all variables that refer to block numbers are uint16), but the code needs to be audited for that

- Logging is too verbose, it should be optionally turned on


## What to do if we actually wanted to use this in production

- unit tests !
- maybe we do want to have an all-memory tftp server, but if now, then FileManager should handle FS files.
- the admin REST interface, albeit useful, should be restricted in production (especially the /clear endpoint!)
- configuration could be read from an external source

