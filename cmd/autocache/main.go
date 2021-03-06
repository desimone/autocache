package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/groupcache"
	"github.com/pomerium/autocache"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAddr = ":http"
)

var exampleCache cache

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	existing := []string{}
	if nodes := os.Getenv("NODES"); nodes != "" {
		existing = strings.Split(nodes, ",")
	}

	ac, err := autocache.New(
		&autocache.Options{
			Scheme:    "http",
			Port:      80,
			SeedNodes: existing})
	if err != nil {
		log.Fatal(err)
	}

	exampleCache.group = groupcache.NewGroup("bcrypt", 1<<20, groupcache.GetterFunc(bcryptKey))

	mux := http.NewServeMux()
	mux.Handle("/get/", exampleCache)
	mux.Handle("/_groupcache/", ac.Pool)
	log.Fatal(http.ListenAndServe(addr, mux))

}

// bcryptKey is am arbitrary getter function. In this example, we simply bcrypt
// the key which is useful because bcrypt:
// 		1) takes a long time
//		2) uses a random seed so non-cache results for the same key are obvious
func bcryptKey(ctx context.Context, key string, dst groupcache.Sink) error {
	now := time.Now()
	defer func() {
		log.Printf("bcryptKey/key:%q\ttime:%v", key, time.Since(now))
	}()
	out, err := bcrypt.GenerateFromPassword([]byte(key), 14)
	if err != nil {
		return err
	}
	if err := dst.SetBytes(out); err != nil {
		return err
	}
	return nil
}

type cache struct {
	group *groupcache.Group
}

func (ac cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	key := r.FormValue("key")
	if key == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	now := time.Now()
	defer func() {
		log.Printf("cacheHandler: group[%s]\tkey[%q]\ttime[%v]", ac.group.Name(), key, time.Since(now))
	}()
	var respBody []byte
	if err := ac.group.Get(r.Context(), key, groupcache.AllocatingByteSliceSink(&respBody)); err != nil {
		log.Printf("Get/cache.Get error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}
