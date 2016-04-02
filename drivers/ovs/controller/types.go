package controller

type CreateNetworkRequest struct {
	SubnetName string `json:"subnet_name"`
	CIDR       string `json:"cidr"`
	ID         int    `json:"nw_id"`
	GWAddr     string `json:"gw_id"`
}

func NewCreateNetworkRequest(name, cidr, gw string, id int) *CreateNetworkRequest {
	return &CreateNetworkRequest{
		SubnetName: name,
		CIDR:       cidr,
		ID:         id,
		GWAddr:     gw,
	}
}

type CreateNetworkResponse struct {
	Result int    `json:"result"`
	ErrMsg string `json:"err_msg"`
}

type DeleteNetworkRequest struct {
	ID int `json:"nw_id"`
}

func NewDeleteNetworkRequest(id int) *DeleteNetworkRequest {
	return &DeleteNetworkRequest{
		ID: id,
	}
}

type ListNetworkRequest struct {
	Name string `json:"nw_name"`
}

func NewListNetworkRequest(name string) *ListNetworkRequest {
	return &ListNetworkRequest{
		Name: name,
	}
}

type Network struct {
	ID         int    `json:"nw_id"`
	Name       string `json:"nw_name"`
	Type       string `json:"type"`
	SegID      int    `json:"seg_id"`
	Status     string `json:"status"`
	AdminState string `json:"admin_state"`
}

type ListNetworkResponse struct {
	Result   int       `json:"result"`
	Networks []Network `json:"list_data"`
}

func (lr *ListNetworkResponse) GetResult() int {
	return lr.Result
}

type RequestIPRequest struct {
	HostIP      string `json:"host_name"`
	ContainerID string `json:"container_id"`
	NetworkName string `json:"nw_name"`
}

func NewRequestIPRequest(hostip, containerID, networkName string) *RequestIPRequest {
	return &RequestIPRequest{
		HostIP:      hostip,
		ContainerID: containerID,
		NetworkName: networkName,
	}
}

type RequestIPResponse struct {
	Result      int    `json:"result"`
	ContainerID string `json:"container_id"`
	FixIP       string `json:"fix_ip"`
	SegID       int    `json:"seg_id"`
}

func (r *RequestIPResponse) GetResult() int {
	return r.Result
}

type ReleaseIPRequest struct {
	ContainerID string `json:"container_id"`
	FixIP       string `json:"fix_ip"`
}

func NewReleaseIPRequest(containerID, fixIP string) *ReleaseIPRequest {
	return &ReleaseIPRequest{
		ContainerID: containerID,
		FixIP:       fixIP,
	}
}

type StandardResponse struct {
	Result int    `json:"result"`
	ErrMsg string `json:"err_msg"`
}

func (s *StandardResponse) GetResult() int {
	return s.Result
}

type ResultTrackedResponse interface {
	// GetResult returns the response result field
	GetResult() int
}
