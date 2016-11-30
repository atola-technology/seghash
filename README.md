## seghash

seghash is a free tool that allows to:
- calculate segmented hashes of image
- verify calculated segmented hashes

## How segmented hashing is it different from regular hashing?

With regular hashing, you get a single hash for the entire image.

With segmented hashing, you end up with many hashes of corresponding LBA ranges (chunks) of the image. The sum of these LBA ranges represents the entire image, just not necessarily in sequential order. By validating all hashes in a set you can still prove that the entire image was not modified.

