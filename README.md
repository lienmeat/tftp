In-memory TFTP Server
=====================

This is a simple in-memory TFTP server, implemented in Go.  It is (intended to be)
RFC1350-compliant, but doesn't implement the additions in later RFCs.  In
particular, options are not recognized.

Usage
-----
Build the binary: 

`make build`

Run the binary with:  
`./tftpd -address 0.0.0.0:7070 -logLevel info -logFile tftp.log`  
You can also run with no arguments for usage information.

A "tftp_requests.log" is created/appended to automatically with only get/put file requests
and any packets that are not understood by tftp (cannot be parsed). 

Testing
-------
Run all tests with:  
`make test`

A few helper scripts exist in the tftp/scripts directory for using the tftp
client on MacOS or Ubuntu against this code.  `tftpmany.sh <IP> <PORT>` is useful for
quick and dirty testing of concurrent load and as a full system test.

Additional Information
----------------------
Minimum Go version tested on: 1.11.x