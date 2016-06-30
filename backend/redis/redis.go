package redis

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"regexp"
	"time"

	"github.com/realfake/shrtie"
	redis "gopkg.in/redis.v4"
)

const metaUntil string = ":until"
const metaCount string = ":count"
const metaCreated string = ":created"

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

	// PATH is the prefix and key (exmpl. prefix/Ab2g )
	// the PATH will be expanded with metadata of the key e.g prefix/Ab2g:count
	// All key are stored seperatly, because golangs encodign is rather slow
	// In theory there aren't many Info(...) requests, so this method should be faster
	// This also makes the PATH:counter faster, rather that Unmarshal/Marshal
	path := r.prefix + key

	// Give error handling to redis Pipelined function
	_, err = r.conn.Pipelined(func(pipe *redis.Pipeline) error {
		now := time.Now()
		pipe.Set(path, value, 0)
		pipe.Set(path+metaCreated, now.Unix(), 0)
		if ttl == 0 {
			pipe.Set(path+metaUntil, 0, 0)
			return nil
		}
		pipe.Set(path+metaUntil, now.Add(ttl).Unix(), 0)
		return nil
	})

	if err != nil {
		return ""
	}

	return key
}

func (r Redis) Get(key string) (string, error) {
	// Check if string is not base64, so user cant access meta data
	// Redis is string-escape save
	if escape.MatchString(key) {
		return "", errors.New("Wrong key")
	}

	// Prepare redis pipeline results
	var url *redis.StringCmd
	var until *redis.StringCmd
	_, err := r.conn.Pipelined(func(pipe *redis.Pipeline) error {
		path := r.prefix + key
		url = pipe.Get(path)
		until = pipe.Get(path + metaUntil)
		pipe.Incr(path + metaCount)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Check if the key is expired
	if ttlTo, _ := until.Int64(); ttlTo != 0 && time.Now().Unix() > ttlTo {
		return "", errors.New("TTL exceeded")
	}

	return url.Val(), nil
}

func (r Redis) Info(key string) (*shrtie.Metadata, error) {
	// Prepare redis pipeline results
	var url *redis.StringCmd
	var untilRaw *redis.StringCmd
	var clickedRaw *redis.StringCmd
	var createdRaw *redis.StringCmd
	_, err := r.conn.Pipelined(func(pipe *redis.Pipeline) error {
		path := r.prefix + key
		url = pipe.Get(path)
		untilRaw = pipe.Get(path + metaUntil)
		clickedRaw = pipe.Get(path + metaCount)
		createdRaw = pipe.Get(path + metaCreated)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert to underlaying values
	// Errors are ignored because the values should be type safe
	until, _ := untilRaw.Int64()
	clicked, _ := clickedRaw.Int64()
	created, _ := createdRaw.Int64()

	// Check if entry TTL is exceeded
	if ttl := until - time.Now().Unix(); ttl < 0 {
		return &shrtie.Metadata{
			Url:     url.Val(),
			TTL:     ttl,
			Clicked: clicked,
			Created: time.Unix(created, 0),
		}, nil
	}
	return nil, errors.New("TTL exceeded")
}
