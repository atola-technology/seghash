## seghash

seghash is a free tool that allows to:
- calculate segmented hashes of image
- verify calculated segmented hashes

Supported hash types: MD5, SHA1, SHA224, SHA256, SHA384, SHA512

## How segmented hashing is different from regular hashing?

With regular hashing, you get a single hash for the entire image.

With segmented hashing, you end up with many hashes of corresponding LBA ranges (chunks) of the image. The sum of these LBA ranges represents the entire image, just not necessarily in sequential order. By validating all hashes in a set you can still prove that the entire image was not modified.

Read more in [Segmented Hashes Whitepaper](https://github.com/atola-technology/seghash/blob/master/Segmented%20Hashes%20White%20Paper.pdf)

## Download binaries

- [Windows 32-bit EXE](http://dl.atola.com/seghash/win32/seghash.exe)
- [Windows 64-bit EXE](http://dl.atola.com/seghash/win64/seghash.exe)
- [Linux](http://dl.atola.com/seghash/linux/seghash)
- [macOS](http://dl.atola.com/seghash/linux/macos)

## Download and compile sources

You can skip steps 1 and 2 if you did them previously.

1. Download and install Go: https://golang.org/doc/install
2. Perform these instructions: https://golang.org/doc/install#testing
3. Run `go get github.com/atola-technology/seghash`

Results:

Source files will be downloaded to the folder: GOPATH/src/github.com/atola-technology/seghash

Binary executable file will be compiled to the folder:  GOPATH/bin

## Examples 

Segmented hashes calculation:

`seghash calc Drive.img sha1`


Segmented hashes verification:

`seghash verify Drive.img Hashes-sha1.csv`
