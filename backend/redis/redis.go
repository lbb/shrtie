package redis

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/realfake/shrtie"
	redis "gopkg.in/redis.v4"
)

const (
	metaUntil   string = "until"
	metaCount          = "count"
	metaCreated        = "created"
	metaURL            = "url"
)

const maxLength = 2048

var (
	ErrWrongKey error = errors.New("Wrong key")
	ErrTTL            = errors.New("TTL exceeded")
)

type Redis struct {
	conn   *redis.Client
	prefix string
}

var escape = regexp.MustCompile(`[^0-9A-Za-z_-]`)

func New(options *redis.Options) (shrtie.GetSaver, error) {
	client := redis.NewClient(options)

	// Test connection
	if _, err := client.Ping().Result(); err != nil {
		return nil, err
	}
	return Redis{
		conn:   client,
		prefix: "shrtie/",
	}, nil
}

func (r Redis) Save(value string, ttl time.Duration) string {
	if len(value) > maxLength {
		return ""
	}
	// Get atomic identifier from the counter
	index, err := r.conn.Incr(r.prefix + "meta:count").Result()
	if err != nil {
		return ""
	}

	// Make int64 to byte array and cut it to min lenght
	buf := make([]byte, 8)
	size := binary.PutVarint(buf, index)

	// Convert to base64, wich is URL save and without padding ('='*)
	key := base64.RawStdEncoding.EncodeToString(buf[:size])

	// Take timestamp
	now := time.Now()

	var until string
	if ttl == 0 {
		until = "0"
	} else {
		until = strconv.FormatInt(now.Add(ttl).Unix(), 10)
	}

	err = r.conn.HMSet(r.prefix+key, map[string]string{
		metaURL:     value,
		metaCreated: strconv.FormatInt(now.Unix(), 10),
		metaUntil:   until,
	}).Err()

	if err != nil {
		return ""
	}

	return key
}

func (r Redis) Get(key string) (string, error) {
	// Check if string is not base64, so user cant access meta data
	// Redis is string-escape save
	if escape.MatchString(key) {
		return "", ErrWrongKey
	}

	// Prepare redis pipeline results
	path := r.prefix + key
	var url *redis.StringCmd
	var until *redis.StringCmd
	_, err := r.conn.Pipelined(func(pipe *redis.Pipeline) error {
		url = pipe.HGet(path, metaURL)
		until = pipe.HGet(path, metaUntil)
		pipe.HIncrBy(path, metaCount, 1)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Check if the key is expired
	if ttlTo, _ := until.Int64(); ttlTo != 0 && time.Now().Unix() > ttlTo {
		return "", ErrTTL
	}

	return url.Val(), nil
}

func (r Redis) Info(key string) (*shrtie.Metadata, error) {
	// path var was used for clearity, can also be omitted
	path := r.prefix + key

	// Get all entrys for this hashtable
	objMap, err := r.conn.HGetAll(path).Result()

	if err != nil {
		return nil, err
	}

	if len(objMap) == 0 {
		return nil, ErrWrongKey
	}

	// Convert to underlaying values
	// Internally redis.v4 also uses strconv
	// Errors are ignored because the values should be safe
	// Check if entry TTL is exceeded
	var ttl int64
	var now = time.Now().Unix()
	if until, _ := strconv.ParseInt(objMap[metaUntil], 10, 64); until-now > 0 {
		ttl = until - now
	} else if until == 0 {
		ttl = 0
	} else {
		return nil, ErrTTL
	}

	//Convert these values afterwards to save process time if ttl is exceeded
	created, _ := strconv.ParseInt(objMap[metaCreated], 10, 64)

	// This can return an error if it wasnt clicked before but
	// doesn't matter because it still returns 0
	clicked, _ := strconv.ParseInt(objMap[metaCount], 10, 64)

	return &shrtie.Metadata{
		URL:     objMap[metaURL],
		TTL:     ttl,
		Clicked: clicked,
		Created: time.Unix(created, 0),
	}, nil
}
