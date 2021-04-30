package main

import (
	"sync"
	"testing"
	gocheck "gopkg.in/check.v1"
)

func TestBalancer(t *testing.T) { gocheck.TestingT(t) }

type MySuiteBalancer struct{}

var _ = gocheck.Suite(&MySuiteBalancer{})

func (s *MySuiteBalancer) TestBalancerInitialization(c *gocheck.C) {
	mockServers   := []string{
		"server1:8080",
		"server2:8080",
		"server3:8080",
	}
	initialized, _ := Initialize(mockServers)

	c.Assert(initialized, gocheck.DeepEquals, &leastConnections{
		servers: []*server{
			{host: "server1:8080", counter: 0, status: true},
			{host: "server2:8080", counter: 0, status: true},
			{host: "server3:8080", counter: 0, status: true},
		},
		mutex: new(sync.Mutex),
	})
}

func (s *MySuiteBalancer) TestBalancing(c *gocheck.C) {
	mockLeastConnections := &leastConnections{
		servers: []*server{
			{host: "server1:8080", counter: 3, status: true},
			{host: "server2:8080", counter: 2, status: true},
			{host: "server3:8080", counter: 6, status: true},
		},
		mutex: new(sync.Mutex),
	}

	server2, restore2, _ := mockLeastConnections.getLeastConnected()

	c.Assert(server2, gocheck.DeepEquals, &server{
		host: "server2:8080",
		counter: 3,
		status: true,
	})

	server1, restore1, _ := mockLeastConnections.getLeastConnected()

	c.Assert(server1, gocheck.DeepEquals, &server{
		host: "server1:8080",
		counter: 4,
		status: true,
	})

	restore2()

	c.Assert(server2, gocheck.DeepEquals, &server{
		host: "server2:8080",
		counter: 2,
		status: true,
	})

	mockNoServers := &leastConnections{
		servers: []*server{
			{host: "server1:8080", counter: 3, status: false},
			{host: "server2:8080", counter: 2, status: false},
			{host: "server3:8080", counter: 6, status: false},
		},
		mutex: new(sync.Mutex),
	}

	_, _, err := mockNoServers.getLeastConnected()

	c.Assert(err, gocheck.ErrorMatches, "no servers online")

	restore1()

}
