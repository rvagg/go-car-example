package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	blocks "github.com/ipfs/go-block-format"
	bsrv "github.com/ipfs/go-blockservice"
	car "github.com/ipfs/go-car"
	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"

	// leveldbds "github.com/ipfs/go-ds-leveldb"
	flatfsds "github.com/ipfs/go-ds-flatfs"
	cbor "github.com/ipfs/go-ipld-cbor"
	multihash "github.com/multiformats/go-multihash"

	ipld "github.com/ipld/go-ipld-prime"
	dagcbor "github.com/ipld/go-ipld-prime/encoding/dagcbor"
	dagjson "github.com/ipld/go-ipld-prime/encoding/dagjson"
	ipldfluent "github.com/ipld/go-ipld-prime/fluent"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"

	num2words "github.com/divan/num2words"
)

func main() {
	{

		// ---------------------- //
		// Create a new .car file //
		// ---------------------- //

		// Datastore does raw Put(key, data) where the keys are in Datastore-specific form, not very friendly
		dataStoreIn, err := createDataStore("datastore.in")
		if err != nil {
			panic(err)
		}
		defer dataStoreIn.Close()

		// Blockstore will do a nice `Put(block.Cid, block.RawData())` for us as a wraper around the Datastore
		blockStoreIn := blockstore.NewBlockstore(dataStoreIn)

		// Create some data using ipld-cbor
		classicRoot, err := createIpldClassicData(blockStoreIn)
		if err != nil {
			panic(err)
		}

		// Create some example IPLD data with ipld-prime
		primeRoot, err := createIpldPrimeData(blockStoreIn)
		if err != nil {
			panic(err)
		}

		// Make a .car file with two roots, one for each of our sets of data, it will slurp out the merkle dag from the
		// Datastore for those two roots
		if err = writeCar("example.car", blockStoreIn, []cid.Cid{*classicRoot, *primeRoot}); err != nil {
			panic(err)
		}

		if err = dataStoreIn.Close(); err != nil {
			panic(err)
		}
	}

	// ----------------------------- //
	// Read in an existing .car file //
	// ----------------------------- //

	{
		// Create a fresh Datastore that we can load .car data into
		dataStoreOut, err := createDataStore("datastore.out")
		if err != nil {
			panic(err)
		}
		defer dataStoreOut.Close()
		blockStoreOut := blockstore.NewBlockstore(dataStoreOut)

		// Load from the .car file, it will extract each block and insert it into the
		// Datastore/Blockstore/Blockservice we supply
		roots, err := readCar("example.car", blockStoreOut)
		if err != nil {
			panic(err)
		}

		// Demonstrate we have the data we stored
		for idx, root := range roots {
			fmt.Printf("example.car header root %v: %v\n", idx+1, root)
		}

		// Print out the blocks(s) we created with ipld-cbor
		if err = dumpIpldClassic(blockStoreOut, roots[0]); err != nil {
			panic(err)
		}

		// Print out the blocks(s) we created with ipld-prime
		if err = dumpIpldPrime(blockStoreOut, roots[1]); err != nil {
			panic(err)
		}
	}
}

// Given a file path/name, a Blockstore and an array of root CIDs, extract the merkle DAGs
// from that Blockstore from those roots and store them in a .car file by the name given
func writeCar(path string, blockStore blockstore.Blockstore, roots []cid.Cid) error {
	// go-car uses DAGService for traversing from the roots; in turn it needs a BlockService
	// which we can instantiate in offline mode
	blockService := bsrv.New(blockStore, offline.Exchange(blockStore))
	dagService := dag.NewDAGService(blockService)

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	outWriter := bufio.NewWriter(outFile)

	if err = car.WriteCar(context.Background(), dagService, roots, outWriter); err != nil {
		return err
	}

	outWriter.Flush()

	return nil
}

// Given a file pathname and a Blockstore, extract the blocks in the .car file into that
// Blockstore and return the root CIDs listed in the .car file header.
func readCar(path string, blockStore blockstore.Blockstore) ([]cid.Cid, error) {
	inFile, err := os.Open("example.car")
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	inReader := bufio.NewReader(inFile)

	header, err := car.LoadCar(blockStore, inReader)
	if err != nil {
		return nil, err
	}

	return header.Roots, nil
}

func createDataStore(path string) (datastore.Batching, error) {
	return flatfsds.CreateOrOpen(path, flatfsds.Suffix(2), true)
	// could also use leveldb:
	// dataStoreOut, err := leveldbds.NewDatastore(path), &leveldbds.Options{})
}

//---------------------------------------------
// Classic ipld-cbor data manipulation
//---------------------------------------------

type cborTest struct {
	S string
	I int
	B bool
}

func createIpldClassicData(blockStore blockstore.Blockstore) (*cid.Cid, error) {
	cbor.RegisterCborType(cborTest{})

	cnd1, err := cbor.WrapObject(cborTest{"foo", 100, false}, multihash.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	if err = blockStore.Put(cnd1); err != nil {
		return nil, err
	}

	cid := cnd1.Cid()
	return &cid, nil
}

func dumpIpldClassic(blockStore blockstore.Blockstore, root cid.Cid) error {
	blkOut, err := blockStore.Get(root)
	if err != nil {
		return err
	}

	var out = cborTest{}
	err = cbor.DecodeInto(blkOut.RawData(), &out)
	if err != nil {
		return err
	}

	fmt.Printf("Decoded classic block [%v]:\n\t%v\n", blkOut.Cid(), out)

	return nil
}

//---------------------------------------------
// New ipld-prime hotness data manipulation
//---------------------------------------------

func createIpldPrimeData(blockStore blockstore.Blockstore) (*cid.Cid, error) {
	var err error
	var lnk ipld.Link
	var fnb = ipldfluent.WrapNodeBuilder(ipldfree.NodeBuilder())

	linkBuilder := cidlink.LinkBuilder{cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}}

	var blockBuilder = func(node ipld.Node) (ipld.Link, []byte, error) {
		buf := bytes.Buffer{}
		lnk, err = linkBuilder.Build(context.Background(), ipld.LinkContext{}, node,
			func(ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
				return &buf, func(lnk ipld.Link) error { return nil }, nil
			},
		)
		if err != nil {
			return nil, nil, err
		}

		return lnk, buf.Bytes(), nil
	}

	// a linked list of blocks each referring to the previous
	for i := 0; i < 10; i++ {
		// create a map with the shape: { number: "one", previous: CID }
		node := fnb.CreateMap(func(mb ipldfluent.MapBuilder, knb ipldfluent.NodeBuilder, vnb ipldfluent.NodeBuilder) {
			mb.Insert(knb.CreateString("number"), vnb.CreateString(num2words.Convert(i)))
			if lnk != nil {
				mb.Insert(knb.CreateString("previous"), vnb.CreateLink(lnk))
			}
		})

		lnk, buf, err := blockBuilder(node)

		cidLink, _ := lnk.(cidlink.Link)
		blk, err := blocks.NewBlockWithCid(buf, cidLink.Cid)
		if err != nil {
			return nil, err
		}
		blockStore.Put(blk)
	}

	cidLink, _ := lnk.(cidlink.Link)
	return &cidLink.Cid, nil
}

// Note that we could use this same technique to print out the block created with ipld-cbor
func dumpIpldPrime(blockStore blockstore.Blockstore, root cid.Cid) error {
	var lnk ipld.Link
	lnk = cidlink.Link{root}

	linkLoader := func(lnk ipld.Link, _ ipld.LinkContext) (io.Reader, error) {
		cidLink, _ := lnk.(cidlink.Link)
		blk, err := blockStore.Get(cidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(blk.RawData()), nil
	}

	for {
		loadedNode, err := lnk.Load(context.Background(), ipld.LinkContext{}, ipldfree.NodeBuilder(), linkLoader)
		if err != nil {
			panic(err)
		}

		number, err := loadedNode.TraverseField("number")
		if err != nil {
			panic(err)
		}
		numberString, err := number.AsString()
		if err != nil {
			panic(err)
		}

		cidLink, _ := lnk.(cidlink.Link)
		fmt.Printf("ipld-prime block [%v]\n\tnumber: %v\n", cidLink.Cid, numberString)

		loadedNode, err = loadedNode.TraverseField("previous")
		if err != nil {
			break
		}

		if loadedNode.ReprKind() != ipld.ReprKind_Link {
			panic("`previous` property is not a link")
		}

		lnk, err = loadedNode.AsLink()
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func init() {
	// the default prefix of "blocks" makes a directory that flatfs datastore doesn't handle properly
	blockstore.BlockPrefix = datastore.NewKey("")
	_ = dagcbor.Encoder // force registration of dagcbor with the cidlink encoder
	_ = dagjson.Encoder // force registration of dagjson with the cidlink encoder
}
