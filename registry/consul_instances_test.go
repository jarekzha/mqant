package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	consul "github.com/hashicorp/consul/api"
)

type consulInstanceTransport struct {
	live          map[string]bool
	registrations map[string]int
	services      map[string]consul.AgentServiceRegistration
}

func (r *consulInstanceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := http.StatusOK
	switch {
	case req.URL.Path == "/v1/agent/service/register":
		var service consul.AgentServiceRegistration
		if err := json.NewDecoder(req.Body).Decode(&service); err != nil {
			return nil, err
		}
		r.services[service.ID] = service
		r.live[service.ID] = true
		r.registrations[service.ID]++
	case req.URL.Path == "/v1/health/service/Gate":
		var entries []*consul.ServiceEntry
		for id, service := range r.services {
			if r.live[id] {
				entries = append(entries, &consul.ServiceEntry{Service: &consul.AgentService{
					ID: id, Service: service.Name, Meta: service.Meta, Tags: service.Tags, Address: service.Address,
				}})
			}
		}
		body, err := json.Marshal(entries)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
	case strings.HasPrefix(req.URL.Path, "/v1/agent/service/deregister/"):
		delete(r.live, strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/"))
	case strings.HasPrefix(req.URL.Path, "/v1/agent/check/pass/service:"):
		id := strings.TrimPrefix(req.URL.Path, "/v1/agent/check/pass/service:")
		if !r.live[id] {
			status = http.StatusNotFound
		}
	default:
		return nil, fmt.Errorf("unexpected Consul request: %s", req.URL.Path)
	}
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
}

func TestConsulRegistrationHashesArePerInstance(t *testing.T) {
	transport := &consulInstanceTransport{live: map[string]bool{}, registrations: map[string]int{}, services: map[string]consul.AgentServiceRegistration{}}
	config := consul.DefaultConfig()
	config.Address = "127.0.0.1:8500"
	config.HttpClient = &http.Client{Transport: transport}
	client, err := consul.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	reg := &consulRegistry{Client: client, register: make(map[string]uint64)}
	first := &Service{Name: "Gate", Version: "v1", Nodes: []*Node{{ServerID: "Gate@Gate001", InstanceID: "Gate@Gate001#A", StartedAt: 100, Address: "inbox-A"}}}
	second := &Service{Name: "Gate", Version: "v1", Nodes: []*Node{{ServerID: "Gate@Gate001", InstanceID: "Gate@Gate001#B", StartedAt: 200, Address: "inbox-B"}}}

	for _, service := range []*Service{first, second, first} {
		if err := reg.Register(service, RegisterTTL(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if transport.registrations[first.Nodes[0].InstanceID] != 1 || transport.registrations[second.Nodes[0].InstanceID] != 1 {
		t.Fatalf("renewal overwrote another instance's hash: %v", transport.registrations)
	}
	services, err := reg.GetService("Gate")
	if err != nil || len(services) != 1 || len(services[0].Nodes) != 2 {
		t.Fatalf("discovery lost instances: %v, %v", services, err)
	}
	for _, node := range services[0].Nodes {
		expected := int64(100)
		if node.InstanceID == second.Nodes[0].InstanceID {
			expected = 200
		}
		if node.StartedAt != expected {
			t.Fatalf("startup time lost: %+v", node)
		}
		if node.ServerID != "Gate@Gate001" || !transport.live[node.InstanceID] {
			t.Fatalf("discovery lost typed identities: %+v", node)
		}
	}
	if err := reg.Deregister(first); err != nil {
		t.Fatal(err)
	}
	if transport.live[first.Nodes[0].InstanceID] || !transport.live[second.Nodes[0].InstanceID] {
		t.Fatalf("old deregistration affected new instance: %v", transport.live)
	}
	if _, ok := reg.register[second.Nodes[0].InstanceID]; !ok {
		t.Fatal("deregistration removed another instance's renewal hash")
	}
	if err := reg.Register(second, RegisterTTL(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if transport.registrations[second.Nodes[0].InstanceID] != 1 {
		t.Fatal("new instance had to re-register after old instance left")
	}
}
