package seeding

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/everFinance/goar/types"
	bolt "go.etcd.io/bbolt"
	"os"
	"path"
	"time"
)

const (
	boltAllocSize = 8 * 1024 * 1024

	dirPath  = "./store"
	boltName = "seed.db"
)

var (
	ErrNotExist = errors.New("not exist")

	// bucket
	ChunkBucket           = []byte("chunk-bucket")              // key: chunkStartOffset, val: chunk
	TxDataEndOffSetBucket = []byte("tx-data-end-offset-bucket") // key: dataRoot+dataSize; val: txDataEndOffSet
	TxMetaBucket          = []byte("tx-meta-bucket")            // key: txId, val: arTx; not include data
	ConstantsBucket       = []byte("constants-bucket")
)

type Store struct {
	BoltDb *bolt.DB
}

func NewStore(bucketNames ...[]byte) (*Store, error) {
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return nil, err
	}

	boltDB, err := bolt.Open(path.Join(dirPath, boltName), 0660, &bolt.Options{Timeout: 1 * time.Second, InitialMmapSize: 10e6})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}
	boltDB.AllocSize = boltAllocSize

	kv := &Store{
		BoltDb: boltDB,
	}

	// create bucket
	if err := kv.BoltDb.Update(func(tx *bolt.Tx) error {
		return createBuckets(tx, bucketNames...)
	}); err != nil {
		return nil, err
	}

	return kv, nil
}

func (s *Store) Close() error {
	return s.BoltDb.Close()
}

func createBuckets(tx *bolt.Tx, buckets ...[]byte) error {
	for _, bucket := range buckets {
		if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SaveAllDataEndOffset(allDataEndOffset uint64) error {
	key := []byte("allDataEndOffset")
	val := itob(allDataEndOffset)
	return s.BoltDb.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(ConstantsBucket)
		return bkt.Put(key, val)
	})
}

func (s *Store) LoadAllDataEndOffset() (offset uint64) {
	key := []byte("allDataEndOffset")
	_ = s.BoltDb.View(func(tx *bolt.Tx) error {
		val := tx.Bucket(ConstantsBucket).Get(key)
		if val == nil {
			offset = 0
		} else {
			offset = btoi(val)
		}
		return nil
	})
	return
}

func (s *Store) SaveTxMeta(arTx types.Transaction) error {
	arTx.Data = "" // only store tx meta, not include data
	key := []byte(arTx.ID)
	val, err := json.Marshal(&arTx)
	if err != nil {
		return err
	}
	return s.BoltDb.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(TxMetaBucket)
		return bkt.Put(key, val)
	})
}

func (s *Store) LoadTxMeta(arId string) (arTx *types.Transaction, err error) {
	key := []byte(arId)
	err = s.BoltDb.View(func(tx *bolt.Tx) error {
		val := tx.Bucket(TxMetaBucket).Get(key)
		if val == nil {
			return ErrNotExist
		} else {
			return json.Unmarshal(val, arTx)
		}
	})
	return
}

func (s *Store) IsExistTxMeta(arId string) bool {
	_, err := s.LoadTxMeta(arId)
	if err == ErrNotExist {
		return true
	}
	return false
}

func (s *Store) SaveTxDataEndOffSet(dataRoot, dataSize string, txDataEndOffset uint64) error {
	return s.BoltDb.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(TxDataEndOffSetBucket)
		return bkt.Put(generateOffSetKey(dataRoot, dataSize), itob(txDataEndOffset))
	})
}

func (s *Store) LoadTxDataEndOffSet(dataRoot, dataSize string) (txDataEndOffset uint64, err error) {
	err = s.BoltDb.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(TxDataEndOffSetBucket)
		val := bkt.Get(generateOffSetKey(dataRoot, dataSize))
		if val == nil {
			return ErrNotExist
		} else {
			txDataEndOffset = btoi(val)
		}
		return nil
	})
	return
}

func (s *Store) IsExistTxDataEndOffset(dataRoot, dataSize string) bool {
	_, err := s.LoadTxDataEndOffSet(dataRoot, dataSize)
	if err == ErrNotExist {
		return true
	}
	return false
}

func (s *Store) SaveChunk(chunkStartOffset uint64, chunk types.GetChunk) error {
	chunkJs, err := chunk.Marshal()
	if err != nil {
		return err
	}
	err = s.BoltDb.Update(func(tx *bolt.Tx) error {
		chunkBkt := tx.Bucket(ChunkBucket)
		if err := chunkBkt.Put(itob(chunkStartOffset), chunkJs); err != nil {
			return err
		}
		return nil
	})

	return err
}

func (s *Store) LoadChunk(chunkStartOffset uint64) (chunk *types.GetChunk, err error) {
	err = s.BoltDb.View(func(tx *bolt.Tx) error {
		chunkBkt := tx.Bucket(ChunkBucket)
		val := chunkBkt.Get(itob(chunkStartOffset))
		if val == nil {
			err = ErrNotExist
		} else {
			err = json.Unmarshal(val, chunk)
		}
		return nil
	})

	return
}

func (s *Store) IsExistChunk(chunkStartOffset uint64) bool {
	_, err := s.LoadChunk(chunkStartOffset)
	if err == ErrNotExist {
		return true
	}
	return false
}

// itob returns an 64-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 64)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func btoi(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

func generateOffSetKey(dataRoot, dataSize string) []byte {
	hash := sha256.Sum256([]byte(dataRoot + dataSize))
	return hash[:]
}
