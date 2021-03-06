package poc

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
)

var (
	host = "127.0.0.0.1"
)

func TestMain(m *testing.M) {
	_ = logging.SetLogLevel("foldersync", "debug")
	_ = logging.SetLogLevel("watcher", "debug")
	os.Exit(m.Run())
}

func TestSimple(t *testing.T) {
	if os.Getenv("SKIP_FOLDERSYNC") != "" {
		t.Skip("Skipping foldersync tests")
	}

	id := thread.NewIDV1(thread.Raw, 32)

	// db0

	repoPath0, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network0, err := newNetwork(repoPath0, host)
	checkErr(t, err)

	store0, err := util.NewBadgerDatastore(repoPath0, "eventstore", false)
	checkErr(t, err)
	defer store0.Close()
	db0, err := db.NewDB(context.Background(), store0, network0, id, db.WithNewCollections(cc))
	checkErr(t, err)
	defer db0.Close()

	c0 := db0.GetCollection(collectionName)

	info0, err := db0.GetDBInfo()
	checkErr(t, err)

	// db1

	repoPath1, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network1, err := newNetwork(repoPath1, host)
	checkErr(t, err)

	store1, err := util.NewBadgerDatastore(repoPath1, "eventstore", false)
	checkErr(t, err)
	defer store1.Close()
	db1, err := db.NewDBFromAddr(
		context.Background(),
		store1,
		network1,
		info0.Addrs[0],
		info0.Key,
		db.WithNewCollections(cc),
	)
	checkErr(t, err)
	defer db1.Close()

	c1 := db1.GetCollection(collectionName)

	// db2

	repoPath2, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network2, err := newNetwork(repoPath2, host)
	checkErr(t, err)

	store2, err := util.NewBadgerDatastore(repoPath2, "eventstore", false)
	checkErr(t, err)
	defer store2.Close()
	db2, err := db.NewDBFromAddr(
		context.Background(),
		store2,
		network2,
		info0.Addrs[0],
		info0.Key,
		db.WithNewCollections(cc),
	)
	checkErr(t, err)
	defer db2.Close()

	c2 := db2.GetCollection(collectionName)

	// db3

	repoPath3, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network3, err := newNetwork(repoPath3, host)
	checkErr(t, err)

	store3, err := util.NewBadgerDatastore(repoPath3, "eventstore", false)
	checkErr(t, err)
	defer store3.Close()
	db3, err := db.NewDBFromAddr(
		context.Background(),
		store3,
		network3,
		info0.Addrs[0],
		info0.Key,
		db.WithNewCollections(cc),
	)
	checkErr(t, err)
	defer db3.Close()

	c3 := db3.GetCollection(collectionName)

	// Add some data

	folder0 := inode{ID: core.NewInstanceID(), Owner: "client0", IsDir: true, IsRoot: true}
	folder1 := inode{ID: core.NewInstanceID(), Owner: "client1", IsDir: true, IsRoot: true}
	folder2 := inode{ID: core.NewInstanceID(), Owner: "client2", IsDir: true, IsRoot: true}
	folder3 := inode{ID: core.NewInstanceID(), Owner: "client3", IsDir: true, IsRoot: true}

	_, err = c0.Create(util.JSONFromInstance(folder0))
	checkErr(t, err)
	_, err = c1.Create(util.JSONFromInstance(folder1))
	checkErr(t, err)
	_, err = c2.Create(util.JSONFromInstance(folder2))
	checkErr(t, err)
	_, err = c3.Create(util.JSONFromInstance(folder3))
	checkErr(t, err)

	time.Sleep(time.Second * 15)

	instances0, err := c0.Find(nil)
	checkErr(t, err)
	if len(instances0) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances0))
	}

	instances1, err := c1.Find(nil)
	checkErr(t, err)
	if len(instances1) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances1))
	}

	instances2, err := c2.Find(nil)
	checkErr(t, err)
	if len(instances2) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances2))
	}

	instances3, err := c3.Find(nil)
	checkErr(t, err)
	if len(instances3) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances3))
	}
}

func TestNUsersBootstrap(t *testing.T) {
	if os.Getenv("SKIP_FOLDERSYNC") != "" {
		t.Skip("Skipping foldersync tests")
	}
	tests := []struct {
		totalClients     int
		totalCorePeers   int
		syncTimeout      int
		randDirsGen      int
		randFilesGen     int
		randFileSize     int
		randRm           int
		checkSyncedFiles bool
	}{
		{totalClients: 2, totalCorePeers: 1, syncTimeout: 15},
		{totalClients: 3, totalCorePeers: 1, syncTimeout: 30},

		{totalClients: 3, totalCorePeers: 2, syncTimeout: 35},

		{totalClients: 2, totalCorePeers: 1, syncTimeout: 20, randFilesGen: 4, randFileSize: 10},
		{totalClients: 3, totalCorePeers: 2, syncTimeout: 35, randFilesGen: 4, randFileSize: 10},
		{totalClients: 3, totalCorePeers: 2, syncTimeout: 35, randDirsGen: 2, randRm: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Total%dCore%d", tt.totalClients, tt.totalCorePeers), func(t *testing.T) {
			var clients []*Client

			client0, clean0 := createRootClient(t, fmt.Sprintf("client%d", 0))
			defer clean0()
			clients = append(clients, client0)

			info, err := client0.getDBInfo()
			checkErr(t, err)

			for i := 1; i < tt.totalCorePeers; i++ {
				client, clean := createJoinerClient(t, fmt.Sprintf("client%d", i), info.Addrs[0], info.Key)
				defer clean()
				clients = append(clients, client)
			}

			for i := tt.totalCorePeers; i < tt.totalClients; i++ {
				info, err := clients[i%tt.totalCorePeers].getDBInfo()
				checkErr(t, err)

				client, clean := createJoinerClient(t, fmt.Sprintf("client%d", i), info.Addrs[0], info.Key)
				defer clean()
				clients = append(clients, client)
			}

			for i := 0; i < tt.totalClients; i++ {
				err := clients[i].start()
				checkErr(t, err)
			}

			blk := make([]byte, tt.randFileSize)
			for i := 0; i < tt.randFilesGen; i++ {
				for j, c := range clients {
					//Test create events
					prefix := path.Join(c.rootPath, c.name)
					fd, err := ioutil.TempFile(prefix, fmt.Sprintf("client%d-", j))
					checkErr(t, err)
					_, err = rand.Read(blk)
					checkErr(t, err)
					_, err = fd.Write(blk)
					checkErr(t, err)
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))
					//Test write events
					_, err = rand.Read(blk)
					checkErr(t, err)
					_, err = fd.Write(blk)
					checkErr(t, err)
					err = fd.Close()
					checkErr(t, err)
				}
			}

			for i := 0; i < tt.randDirsGen; i++ {
				for j, c := range clients {
					//Test create events
					prefix := path.Join(c.rootPath, c.name)
					_, err := ioutil.TempDir(prefix, fmt.Sprintf("client%d-", j))
					checkErr(t, err)
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))

					// TODO: fs watcher only monitor rootpath
					/*fd, err := os.OpenFile(path+"/test", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
					checkErr(t, err)
					_, err = rand.Read(blk)
					checkErr(t, err)
					_, err = fd.Write(blk)
					checkErr(t, err)
					err = fd.Close()
					checkErr(t, err)
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))*/
				}
			}

			i := 0
			for ; i < tt.syncTimeout; i++ {
				time.Sleep(time.Second)
				if checkClientsEqualTrees(t, clients) {
					break
				}
			}

			if i == tt.syncTimeout {
				t.Fatalf("trees from clients aren't equal")
			}

			if tt.randRm != 0 {
				for i = 0; i < tt.randRm; i++ {
					for _, c := range clients {
						//Test remove events
						prefix := path.Join(c.rootPath, c.name)
						dirs, err := os.ReadDir(prefix)
						checkErr(t, err)
						if len(dirs) > 0 {
							fmt.Println("remove ", path.Join(prefix, dirs[0].Name()))
							err := os.Remove(path.Join(prefix, dirs[0].Name()))
							checkErr(t, err)
							time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))
						}
					}
				}

				i = 0
				for ; i < tt.syncTimeout; i++ {
					time.Sleep(time.Second)
					if checkClientsEqualTrees(t, clients) {
						break
					}
				}

				if i == tt.syncTimeout {
					t.Fatalf("trees from clients aren't equal")
				}
			}
		})
	}
}

func checkClientsEqualTrees(t *testing.T, clients []*Client) bool {
	totalClients := len(clients)
	trees := make([][]*inode, totalClients)
	for i := 0; i < totalClients; i++ {
		tree, err := clients[i].getDirectoryTree()
		checkErr(t, err)
		trees[i] = tree
	}

	return equalTrees(trees)
}

func equalInode(i1, i2 *inode) bool {
	return i1.ID == i2.ID && i1.CID == i2.CID && i1.Path == i2.Path
}

func equalTrees(trees [][]*inode) bool {
	baseTree := trees[0]

	for i := 1; i < len(trees); i++ {
		t := trees[i]
		if len(baseTree) != len(t) {
			return false
		}

		// Meta check
		for j := 0; j < len(t); j++ {
			if !equalInode(baseTree[j], t[j]) {
				return false
			}
		}
		// TODO: Need data Check?
	}

	return true
}

func createRootClient(t *testing.T, name string) (*Client, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	client, err := newRootClient(name, folderPath, repoPath, host)
	checkErr(t, err)
	return client, func() {
		fmt.Printf("Closing root client %v\n", client.name)
		err := client.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		os.RemoveAll(folderPath)
		fmt.Printf("Root client %v closed\n", client.name)
	}
}

func createJoinerClient(t *testing.T, name string, addr ma.Multiaddr, key thread.Key) (*Client, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	client, err := newJoinerClient2(name, folderPath, repoPath, host, addr, key)
	checkErr(t, err)
	return client, func() {
		fmt.Printf("Closing joiner client %v\n", client.name)
		err := client.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		os.RemoveAll(folderPath)
		fmt.Printf("Joiner client %v closed\n", client.name)
	}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
