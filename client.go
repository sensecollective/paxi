package paxi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"paxi/glog"
	"strconv"
	"sync"
	"time"
)

// Client main access point of client lib
type Client struct {
	ID        ID
	N         int
	addrs     map[ID]string
	http      map[ID]string
	algorithm string

	index map[Key]ID
	cid   CommandID

	results map[CommandID]bool

	sync.RWMutex
	sync.WaitGroup
}

// NewClient creates a new Client from config
func NewClient(config *Config) *Client {
	c := new(Client)
	c.ID = config.ID
	c.N = len(config.Addrs)
	c.addrs = config.Addrs
	c.http = config.HTTPAddrs
	c.algorithm = config.Algorithm
	c.index = make(map[Key]ID)
	c.results = make(map[CommandID]bool, config.BufferSize)
	return c
}

func (c *Client) RestGet(key Key) Value {
	c.cid++
	c.RLock()
	id, exists := c.index[key]
	c.RUnlock()
	if !exists {
		id = NewID(c.ID.Site(), 1)
	}

	url := c.http[id]
	url += "/" + strconv.Itoa(int(key))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		glog.Errorln(err)
		return nil
	}
	req.Header.Set("id", c.ID.String())
	req.Header.Set("cid", fmt.Sprintf("%v", c.cid))
	rep, err := http.DefaultClient.Do(req)
	if err != nil {
		glog.Errorln(err)
		dump, _ := httputil.DumpRequest(req, true)
		glog.Errorf("%q", dump)
		glog.Errorln(url)
		return nil
	}
	defer rep.Body.Close()
	dump, _ := httputil.DumpResponse(rep, true)
	log.Println(rep.Status)
	log.Printf("%q", dump)
	if rep.StatusCode == http.StatusOK {
		b, _ := ioutil.ReadAll(rep.Body)
		return Value(b)
	}
	return nil
}

func (c *Client) JsonGet(key Key) Value {
	c.cid++
	cmd := Command{GET, key, nil}
	req := new(Request)
	req.ClientID = c.ID
	req.CommandID = c.cid
	req.Command = cmd
	req.Timestamp = time.Now().UnixNano()

	c.RLock()
	id, exists := c.index[key]
	c.RUnlock()
	if !exists {
		id = NewID(c.ID.Site(), 1)
	}

	url := c.http[id]
	data, err := json.Marshal(*req)
	rep, err := http.Post(url, "json", bytes.NewBuffer(data))
	if err != nil {
		glog.Errorln(err)
		return nil
	}
	defer rep.Body.Close()
	if rep.StatusCode == http.StatusOK {
		b, _ := ioutil.ReadAll(rep.Body)
		return Value(b)
	}
	return nil
}

// Get post json get request to server url
func (c *Client) Get(key Key) Value {
	return c.RestGet(key)
}

// GetAsync do Get request in goroutine
func (c *Client) GetAsync(key Key) {
	c.Add(1)
	c.Lock()
	c.results[c.cid+1] = false
	c.Unlock()
	go c.Get(key)
}

func (c *Client) RestPut(key Key, value Value) {
	c.cid++
	c.RLock()
	id, exists := c.index[key]
	c.RUnlock()
	if !exists {
		id = NewID(c.ID.Site(), 1)
	}

	url := c.http[id]
	url += "/" + strconv.Itoa(int(key))

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(value))
	if err != nil {
		glog.Errorln(err)
		dump, _ := httputil.DumpRequest(req, true)
		glog.Errorf("%q", dump)
		glog.Errorln(url)
		return
	}
	req.Header.Set("id", c.ID.String())
	req.Header.Set("cid", fmt.Sprintf("%v", c.cid))
	rep, err := http.DefaultClient.Do(req)
	if err != nil {
		glog.Errorln(err)
		return
	}
	defer rep.Body.Close()
	dump, _ := httputil.DumpResponse(rep, true)
	log.Println(rep.Status)
	log.Printf("%q", dump)
}

func (c *Client) JsonPut(key Key, value Value) {
	c.cid++
	cmd := Command{PUT, key, value}
	req := new(Request)
	req.ClientID = c.ID
	req.CommandID = c.cid
	req.Command = cmd
	req.Timestamp = time.Now().UnixNano()

	c.RLock()
	id, exists := c.index[key]
	c.RUnlock()
	if !exists {
		id = NewID(c.ID.Site(), 1)
	}

	url := c.http[id]
	data, err := json.Marshal(*req)
	rep, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		glog.Errorln(err)
		return
	}
	defer rep.Body.Close()
	dump, _ := httputil.DumpResponse(rep, true)
	log.Println(rep.Status)
	log.Printf("%q", dump)
}

// Put post json request
func (c *Client) Put(key Key, value Value) {
	c.RestPut(key, value)
}

// PutAsync do Put request in goroutine
func (c *Client) PutAsync(key Key, value Value) {
	c.Add(1)
	c.Lock()
	c.results[c.cid+1] = false
	c.Unlock()
	go c.Put(key, value)
}

// RequestDone returns the total number of succeed async reqeusts
func (c *Client) RequestDone() int {
	sum := 0
	for _, succeed := range c.results {
		if succeed {
			sum++
		}
	}
	return sum
}

func (c *Client) Start() {}

func (c *Client) Stop() {}