package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
	discovery "k8s.io/api/discovery/v1beta1"
)

const hostNameLabel = "kubernetes.io/hostname"

type sliceEventHandler struct{ eventHandler }

func serviceNameFrom(eps *discovery.EndpointSlice) string {
	if eps.Labels == nil {
		return ""
	}
	return eps.Labels[discovery.LabelServiceName]
}

func (h sliceEventHandler) OnAdd(obj interface{}) {
	eps := obj.(*discovery.EndpointSlice)

	serviceName := serviceNameFrom(eps)
	if serviceName == "" {
		// no name => not associated with a service => ignore
		return
	}

	// compute endpoints
	infos := make([]*localnetv1.EndpointInfo, 0, len(eps.Endpoints))

	for _, sliceEndpoint := range eps.Endpoints {
		info := &localnetv1.EndpointInfo{
			Namespace:   eps.Namespace,
			ServiceName: serviceName,
			SourceName:  eps.Name,
			Topology:    sliceEndpoint.Topology,
			Endpoint:    &localnetv1.Endpoint{},
			Conditions:  &localnetv1.EndpointConditions{},
		}

		if sliceEndpoint.Topology != nil {
			info.NodeName = sliceEndpoint.Topology[hostNameLabel]
		}

		if h := sliceEndpoint.Hostname; h != nil {
			info.Endpoint.Hostname = *h
		}

		if r := sliceEndpoint.Conditions.Ready; r != nil && *r {
			info.Conditions.Ready = true
		}

		for _, addr := range sliceEndpoint.Addresses {
			info.Endpoint.AddAddress(addr)
		}

		infos = append(infos, info)
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetEndpointsOfSource(eps.Namespace, eps.Name, infos)
		h.updateSync(proxystore.Endpoints, tx)
	})
}

func (h sliceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h sliceEventHandler) OnDelete(oldObj interface{}) {
	eps := oldObj.(*discovery.EndpointSlice)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelEndpointsOfSource(eps.Namespace, eps.Name)
		h.updateSync(proxystore.Endpoints, tx)
	})
}