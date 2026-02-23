package tool_cache

import (
	"fmt"
	"kasper/src/abstract"
	modulelogger "kasper/src/core/module/logger"
	"kasper/src/shell/layer1/adapters"
	"kasper/src/shell/utils/crypto"
	"log"
	"sync"

	"github.com/dgraph-io/badger"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type CacheTrx struct {
	Lock    sync.RWMutex
	Cache   adapters.ICache
	Changes []abstract.CacheUpdate
}

type Cache struct {
	Lock        sync.RWMutex
	Core        abstract.ICore
	logger      *modulelogger.Logger
	Indexes     map[string]map[string]string
	Dict        map[string]string
	RedisClient *redis.Client
	kvdb        *badger.DB
}

func NewCache(core abstract.ICore, logger *modulelogger.Logger, redisUri string) *Cache {
	logger.Println("connecting to cache...")
	opts, err := redis.ParseURL(redisUri)
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(opts)
	kvdb, err := badger.Open(badger.DefaultOptions("/home/ubuntu/storage/basedb").WithSyncWrites(true))
	if err != nil {
		panic(err)
	}
	return &Cache{Core: core, RedisClient: client, kvdb: kvdb, logger: logger}
}

func (m *Cache) DoCacheTrx() adapters.ICacheTrx {
	return &CacheTrx{Cache: m, Changes: []abstract.CacheUpdate{}}
}

func (m *Cache) Infra() any {
	return m.RedisClient
}

func (m *Cache) Put(key string, value string) {
	trx := m.kvdb.NewTransaction(true)
	defer trx.Commit()
	err := trx.Set([]byte(key), []byte(value))
	if err != nil {
		log.Println(err)
	}
}

func (m *Cache) Get(key string) string {
	trx := m.kvdb.NewTransaction(false)
	defer trx.Commit()
	item, err := trx.Get([]byte(key))
	if err != nil {
		return ""
	}
	value := []byte{}
	item.Value(func(val []byte) error {
		value = val
		return nil
	})
	return string(value)
}

func (m *Cache) Keys(mainPart string) []string {
	trx := m.kvdb.NewTransaction(false)
	defer trx.Commit()
	prefix := []byte(mainPart[0:len(mainPart)-1])
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = prefix
	it := trx.NewIterator(opts)
	defer it.Close()
	res := []string{}
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		itemKey := item.Key()
		res = append(res, string(itemKey))
	}
	return res
}

func (m *Cache) Del(key string) {
	trx := m.kvdb.NewTransaction(true)
	defer trx.Commit()
	err := trx.Delete([]byte(key))
	if err != nil {
		log.Println(err)
	}
}

func (m *Cache) GenId(db *gorm.DB, origin string) string {
	if origin == "global" {
		m.Lock.Lock()
		defer m.Lock.Unlock()
		val := abstract.Counter{Id: "globalIdCounter"}
		db.First(&val)
		val.Value = val.Value + 1
		db.Save(&val)
		return fmt.Sprintf("%d@%s", val.Value, origin)
	} else {
		return crypto.SecureUniqueId(m.Core.Id())
	}
}

func (m *CacheTrx) Put(key string, value string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	m.Changes = append(m.Changes, adapters.NewUpdatePut(key, value))
	m.Cache.Put(key, value)
}

func (m *CacheTrx) Del(key string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	m.Changes = append(m.Changes, adapters.NewUpdateDel(key))
	m.Cache.Del(key)
}

func (m *CacheTrx) Updates() []abstract.CacheUpdate {
	return m.Changes
}
