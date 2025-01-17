package loghandler

type CosmosClient struct {
}

func NewCosmosClient() (*CosmosClient, error) {
	return &CosmosClient{}, nil
}

func (c *CosmosClient) Log(resource, action, namespace string, data Data) {

}
