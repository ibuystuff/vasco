package registry

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/AchievementNetwork/vasco/cache"
	"github.com/stretchr/testify/assert"
)

var c cache.Cache
var r *Registry

type server struct {
	name    string
	addr    string
	pattern string
	weight  int
}

var hashes = []string{"", "", "", "", "", "", "", ""}

var servers = []server{
	server{"user", "http://1.1.1.1:8080", "/user", 0},
	server{"tags", "http://1.1.1.1:8081", "/tags", 90},
	server{"user", "http://1.1.1.2:8080", "/user", 0},
	server{"tags", "http://1.1.1.2:8081", "/tags", 10},
	server{"newtags", "http://1.1.1.2:8091", "/tags/extra", 0},
	server{"default", "http://1.1.1.3:8080", "/", 0},
	server{"rewrite", "http://1.1.1.4:8081", "/rewrite(/.*)", 0},
}

func TestMain(m *testing.M) {
	// always starts wiped clean
	if os.Getenv("TEST_REDIS") != "" {
		c = cache.NewRedisCache("localhost:6379")
	} else {
		c = cache.NewLocalCache()
	}

	defer c.Close()
	r = NewRegistry(c, "/tmp", "", 60)

	memResult := m.Run()

	os.Exit(memResult)
}

func makeJson(svr server) string {
	j := fmt.Sprintf(`{
        "name": "%s",
        "address": "%s",
        "pattern": "%s",
        "status": {"path": "/status"},
        "weight": %d
        }`, svr.name, svr.addr, svr.pattern, svr.weight)
	fmt.Println(j)
	return j
}

func TestRegistryBasics(t *testing.T) {
	reg := NewRegFromJSON(makeJson(servers[0]))
	assert.NotNil(t, reg)
	hashes[0] = r.Register(reg, true)
	reg2 := r.Find(hashes[0])
	assert.NotNil(t, reg2)
	assert.Equal(t, reg.Name, reg2.Name)
	assert.Equal(t, reg.Address, reg2.Address)
	assert.Equal(t, reg.Pattern, reg2.Pattern)
	assert.Equal(t, reg.Weight, reg2.Weight)
	assert.Equal(t, reg.Stat, reg2.Stat)
}

func TestRegistryMultiple(t *testing.T) {
	reg := NewRegFromJSON(makeJson(servers[1]))
	assert.NotNil(t, reg)
	hashes[1] = r.Register(reg, true)
	reg2 := r.Find(hashes[1])
	assert.NotNil(t, reg2)
}

func TestMatchSuccess1(t *testing.T) {
	req := "http://testserver.com/user/login"
	regist, err := r.FindBestMatch(req)
	assert.Nil(t, err)
	assert.Equal(t, servers[0].name, regist.Name)
}

func TestMatchSuccess2(t *testing.T) {
	req := "http://testserver.com/tags"
	regist, err := r.FindBestMatch(req)
	assert.Nil(t, err)
	assert.Equal(t, servers[1].name, regist.Name)
}

func TestMatchFail(t *testing.T) {
	req := "http://testserver.com/login"
	_, err := r.FindBestMatch(req)
	assert.NotNil(t, err)
}

func TestUnregister(t *testing.T) {
	reg := r.Find(hashes[0])
	assert.NotNil(t, reg)
	r.Unregister(reg)
	reg = r.Find(hashes[0])
	assert.Nil(t, reg)
}

func TestMatchComplex1(t *testing.T) {
	// #1 gets reregistered here, but that should be fine
	for i := 0; i < len(servers); i++ {
		reg := NewRegFromJSON(makeJson(servers[i]))
		assert.NotNil(t, reg)
		r.Register(reg, true)
	}

	req := "http://testserver.com/user/login"
	regist, err := r.FindBestMatch(req)
	assert.Nil(t, err)
	assert.Equal(t, "user", regist.Name)
}

func TestMatchComplex2(t *testing.T) {
	req := "http://testserver.com/user"
	items := make(map[string]bool)
	for i := 0; i < 10; i++ {
		regist, err := r.FindBestMatch(req)
		assert.Nil(t, err)
		assert.Equal(t, "user", regist.Name)
		items[regist.Address] = true
	}
	assert.Equal(t, 2, len(items))
}

func TestMatchComplex3(t *testing.T) {
	req := "http://testserver.com/tags/extra/whatever"
	for i := 0; i < 10; i++ {
		regist, err := r.FindBestMatch(req)
		assert.Nil(t, err)
		assert.Equal(t, "newtags", regist.Name)
	}
}

func TestMatchComplex4(t *testing.T) {
	// don't match unless it's at the start of the path
	req := "http://testserver.com/something/tags/whatever"
	for i := 0; i < 10; i++ {
		regist, err := r.FindBestMatch(req)
		assert.Nil(t, err)
		assert.Equal(t, "default", regist.Name)
	}
}

func TestMatchComplex5(t *testing.T) {
	// make sure we don't match "/tagserver" to "/tags"
	// we're not doing this correctly yet, but we will come back to it
	t.SkipNow()
	req := "http://testserver.com/tagserver"
	for i := 0; i < 10; i++ {
		regist, err := r.FindBestMatch(req)
		assert.Nil(t, err)
		assert.Equal(t, "default", regist.Name)
	}
}

func TestRouting1(t *testing.T) {
	req, _ := url.Parse("http://testserver.com/user/login")
	err := r.RewriteUrl(req)
	assert.Nil(t, err)
	matched, _ := regexp.MatchString(`http://1\.1\.1\..:8080/user/login`, req.String())
	assert.True(t, matched)
}

func TestRouting2(t *testing.T) {
	req, _ := url.Parse("http://testserver.com/rewrite/login")
	err := r.RewriteUrl(req)
	assert.Nil(t, err)
	matched, _ := regexp.MatchString(`http://1\.1\.1\..:8081/login`, req.String())
	assert.True(t, matched)
}

func TestRoutingRandom(t *testing.T) {
	output := make(map[string]int)
	for i := 0; i < 100; i++ {
		req, _ := url.Parse("http://testserver.com/tags/whatever")
		err := r.RewriteUrl(req)
		assert.Nil(t, err)
		output[req.String()] += 1
	}
	// This will fail randomly sometimes if you get unlucky, but it shouldn't happen
	// so often that it blocks things.
	assert.InDelta(t, 90, output["http://1.1.1.1:8081/tags/whatever"], 7)
	assert.InDelta(t, 9, output["http://1.1.1.2:8081/tags/whatever"], 7)
	fmt.Println(output)
}
