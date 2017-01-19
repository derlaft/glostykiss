==e2.fsck: Root inode is not a directory.  Clear?==

Yeah, kill the root directory! It is *FUN*, isn't it?!

==glostykiss: ext4 lost+found sort pain-killer==

* Usage is as simple as ```go get github.com/derlaft/glostykiss && glostykiss /mnt/lost+found /tmp/database.sqlite```.
* You will need ```<magic.h>``` (libmagic-dev package in Debian-like distros, file-devel in OpenSUSE).
* This will (eventually) create a sqlite3 database with a file index.
* Each file and directory in lost+found will be presented in ```files``` table.
* For each file without an extension (i.e. files without a dot in the name) will have a ```type``` column filled with a mime type obtained by libmagic.
* Use any SQL-queries to find the stuff you need over thousands of unnamed files.
