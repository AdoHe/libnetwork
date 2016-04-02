package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	restful "github.com/emicklei/go-restful"
)

var pool IPPool
var defaultVLANTag = 110
var containerID = "0123456789"

type IPPool struct {
	addresses []string
	allocated map[string]string
}

func init() {
	pool = IPPool{
		addresses: []string{"10.0.0.1", "10.0.0.2"},
		allocated: make(map[string]string),
	}
}

type Controller interface {
	CreateNetwork(name, cidr, gw string, id int) (*StandardResponse, error)
	DeleteNetwork(id int) (*StandardResponse, error)
	ListNetworks(name string) (*ListNetworkResponse, error)

	RequestIP(networkName, containerID string) (*RequestIPResponse, error)
	ReleaseIP(containerID, fixIP string) (*StandardResponse, error)
}

type Server struct {
	controller  Controller
	restfulCont *restful.Container
}

// NewServer initializes and configures a network controller
// server to handle HTTP requests.
func NewServer(controller Controller) Server {
	server := Server{
		controller:  controller,
		restfulCont: restful.NewContainer(),
	}
	server.InstallDefaultHandlers()
	return server
}

// InstallDefaultHandlers registers the default set of supported HTTP request
// patterns with the restful Container.
func (s *Server) InstallDefaultHandlers() {
	var ws *restful.WebService
	ws = new(restful.WebService)
	ws.
		Path("/api/baymax/v1").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("attach_port").
		To(s.getIP).
		Operation("getIP"))

	ws.Route(ws.POST("dettach_port").
		To(s.returnIP).
		Operation("returnIP"))

	ws.Route(ws.POST("create_subnet").
		To(s.createNetwork).
		Operation("createNetwork"))

	ws.Route(ws.POST("delete_subnet").
		To(s.deleteNetwork).
		Operation("deleteNetwork"))

	ws.Route(ws.POST("list_network").
		To(s.listNetworks).
		Operation("list_network"))

	s.restfulCont.Add(ws)
}

func (s *Server) getIP(request *restful.Request, response *restful.Response) {
	req := new(RequestIPRequest)
	err := request.ReadEntity(req)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	requestIPResp, err := s.controller.RequestIP(req.NetworkName, req.ContainerID)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	err = response.WriteHeaderAndEntity(http.StatusOK, requestIPResp)
	if err != nil {
		fmt.Printf("err: %v", err)
	}
}

func (s *Server) returnIP(request *restful.Request, response *restful.Response) {
	req := new(ReleaseIPRequest)
	err := request.ReadEntity(req)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	releaseIPResp, err := s.controller.ReleaseIP(req.ContainerID, req.FixIP)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteHeaderAndEntity(http.StatusOK, releaseIPResp)
}

func (s *Server) createNetwork(request *restful.Request, response *restful.Response) {
	req := new(CreateNetworkRequest)
	err := request.ReadEntity(req)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	createNetworkResp, err := s.controller.CreateNetwork(req.SubnetName, req.CIDR, req.GWAddr, req.ID)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteHeaderAndEntity(http.StatusOK, createNetworkResp)
}

func (s *Server) deleteNetwork(request *restful.Request, response *restful.Response) {
	req := new(DeleteNetworkRequest)
	err := request.ReadEntity(req)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	deleteNetworkResp, err := s.controller.DeleteNetwork(req.ID)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteHeaderAndEntity(http.StatusOK, deleteNetworkResp)
}

func (s *Server) listNetworks(request *restful.Request, response *restful.Response) {
	req := new(ListNetworkRequest)
	err := request.ReadEntity(req)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	listNetworkResp, err := s.controller.ListNetworks(req.Name)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteHeaderAndEntity(http.StatusOK, listNetworkResp)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.restfulCont.ServeHTTP(w, req)
}

type fakeNetworkController struct {
	createNetworkFunc func(name, cidr, gw string, id int) (*StandardResponse, error)
	deleteNetworkFunc func(id int) (*StandardResponse, error)
	listNetworksFunc  func(name string) (*ListNetworkResponse, error)

	allocateIPFunc func(name, id string) (*RequestIPResponse, error)
	recliamIPFunc  func(id, ip string) (*StandardResponse, error)
}

func (fk *fakeNetworkController) CreateNetwork(name, cidr, gw string, id int) (*StandardResponse, error) {
	return fk.createNetworkFunc(name, cidr, gw, id)
}

func (fk *fakeNetworkController) DeleteNetwork(id int) (*StandardResponse, error) {
	return fk.deleteNetworkFunc(id)
}

func (fk *fakeNetworkController) ListNetworks(name string) (*ListNetworkResponse, error) {
	return fk.listNetworksFunc(name)
}

func (fk *fakeNetworkController) RequestIP(networkName, containerID string) (*RequestIPResponse, error) {
	return fk.allocateIPFunc(networkName, containerID)
}

func (fk *fakeNetworkController) ReleaseIP(containerID, fixIP string) (*StandardResponse, error) {
	return fk.recliamIPFunc(containerID, fixIP)
}

// ControllerTestServer encapsulates the datastructures needed to start local
// instance for testing
type ControllerTestServer struct {
	serverUnderTest       *Server
	fakeNetworkController *fakeNetworkController
	testHttpServer        *httptest.Server

	client *Client
}

func (s *ControllerTestServer) terminate() {
	s.client = nil
	s.testHttpServer.CloseClientConnections()
}

func configureTestServer() *ControllerTestServer {
	server := &ControllerTestServer{}
	server.fakeNetworkController = &fakeNetworkController{
		createNetworkFunc: func(name, cidr, gw string, id int) (*StandardResponse, error) {
			return nil, nil
		},
		deleteNetworkFunc: func(id int) (*StandardResponse, error) {
			return nil, nil
		},
		listNetworksFunc: func(name string) (*ListNetworkResponse, error) {
			return nil, nil
		},
		allocateIPFunc: func(name, id string) (*RequestIPResponse, error) {
			for _, addr := range pool.addresses {
				if _, ok := pool.allocated[addr]; !ok {
					pool.allocated[addr] = id
					return &RequestIPResponse{
						Result:      0,
						ContainerID: id,
						FixIP:       addr,
						SegID:       defaultVLANTag,
					}, nil
				}
			}
			return &RequestIPResponse{Result: 1}, nil
		},
		recliamIPFunc: func(id, ip string) (*StandardResponse, error) {
			if cid, ok := pool.allocated[ip]; ok {
				if cid != id {
					return &StandardResponse{
						Result: 1,
						ErrMsg: "container id mismatch",
					}, nil
				}
				delete(pool.allocated, ip)
				return &StandardResponse{Result: 0}, nil
			}
			return &StandardResponse{
				Result: 1,
				ErrMsg: "ip not exist in this pool",
			}, nil
		},
	}
	s := NewServer(server.fakeNetworkController)
	server.serverUnderTest = &s
	server.testHttpServer = httptest.NewServer(server.serverUnderTest)

	return server
}

func NewControllerTestServer(t *testing.T) *ControllerTestServer {
	var err error
	server := configureTestServer()
	server.client, err = NewClient(server.testHttpServer.URL)
	if err != nil {
		t.Errorf("Unexpected error in NewControllerTestServer (%v)", err)
		server.terminate()
		return nil
	}
	return server
}

func TestRequestIP(t *testing.T) {
	server := NewControllerTestServer(t)
	defer server.terminate()
	resp, err := server.client.RequestIP("default", containerID)
	if err != nil {
		t.Fatal("failed to request ip resource %v", err)
	}
	if len(resp.FixIP) == 0 {
		t.Fatalf("get invalid ip address: %s", resp.FixIP)
	}
	if resp.ContainerID != containerID {
		t.Fatalf("expected container ID %s, got %s", containerID, resp.ContainerID)
	}
	if resp.SegID != defaultVLANTag {
		t.Fatalf("expected vlan tag %d, got %d", defaultVLANTag, resp.SegID)
	}
}

func TestReleaseIP(t *testing.T) {
	server := NewControllerTestServer(t)
	defer server.terminate()
	err := server.client.ReleaseIP(containerID, "10.0.0.1")
	if err != nil {
		t.Fatal("failed to release ip resource %v", err)
	}
	if _, ok := pool.allocated["10.0.0.1"]; ok {
		t.Fatal("the ip is still allocated")
	}
}
