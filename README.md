# go-car-example

Example code for interating with:

  * [go-car](https://github.com/ipfs/go-car) - .car content archive format used by Filecoin
  * [go-datastore](https://github.com/ipfs/go-datastore) - implementing a standard key/value storage API based on [this Python thing](https://github.com/datastore/datastore)
  * [go-ds-flatfs](https://github.com/ipfs/go-ds-flatfs) - implementing a filesystem-based datastore
  * [go-ds-leveldb](https://github.com/ipfs/go-ds-leveldb) - implementing a LevelDB-based datastore
  * [go-ipfs-blockstore](https://github.com/ipfs/go-ipfs-blockstore) - wrapping a go-datastore in a slightly nicer interface for IPLD blocks
  * [go-cbor](https://github.com/ipfs/go-ipld-cbor) - for interacting directly with IPLD CBOR blocks
  * [go-ipld-prime](https://github.com/ipld/go-ipld-prime) - the new hotness for interacting with IPLD blocks

`go run example.go`

You get:

* A .car file: example.car
* Stdout showing the header (root CIDs) of a .car file that's generated from example data
* Stdout showing decoding of the blocks in the .car file
* A Datastore (flatfs) that was used as the source for creating the .car file: datastore.in
* A Datastore (flatfs) that was used as the destination for dumping the .car file: datastore.out
