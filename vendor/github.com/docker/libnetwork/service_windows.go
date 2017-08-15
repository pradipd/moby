package libnetwork

import (
	"net"

	"github.com/Microsoft/hcsshim"
	"github.com/docker/docker/pkg/system"
	"github.com/sirupsen/logrus"
)

type policyLists struct {
	ilb *hcsshim.PolicyList
	elb *hcsshim.PolicyList
}

var lbPolicylistMap map[*loadBalancer]*policyLists

func init() {
	lbPolicylistMap = make(map[*loadBalancer]*policyLists)
}

func (n *network) addLBBackend(ip, vip net.IP, lb *loadBalancer, ingressPorts []*PortConfig) {

	if system.GetOSVersion().Build > 16236 {
		//find the load balancer IP for the network.
		var sourceVIP string
		for _, e := range n.Endpoints() {
			epInfo := e.Info()
			if epInfo == nil {
				continue
			}
			sandbox := epInfo.Sandbox()
			if sandbox != nil && sandbox.ContainerID() == n.Name()+"-sbox" {
				sourceVIP = e.Info().Iface().Address().IP.String()
				break
			}
		}

		if sourceVIP == "" {
			logrus.Errorf("Failed to find load balancer IP for network %s", n.Name())
			return
		}

		var endpoints []hcsshim.HNSEndpoint

		for eid := range lb.backEnds {
			//Call HNS to get back ID (GUID) corresponding to the endpoint.
			hnsEndpoint, err := hcsshim.GetHNSEndpointByName(eid)
			if err != nil {
				logrus.Errorf("Failed to find HNS ID for endpoint %v: %v", eid, err)
				return
			}

			endpoints = append(endpoints, *hnsEndpoint)
		}

		if policies, ok := lbPolicylistMap[lb]; ok {

			if policies.ilb != nil {
				policies.ilb.Delete()
				policies.ilb = nil
			}

			if policies.elb != nil {
				policies.elb.Delete()
				policies.elb = nil
			}
			delete(lbPolicylistMap, lb)
		}

		ilbPolicy, err := hcsshim.AddLoadBalancer(endpoints, true, sourceVIP, vip.String(), 0, 0, 0)
		if err != nil {
			logrus.Errorf("Failed to add ILB policy for service %s (%s) with endpoints %v using load balancer IP %s on network %s: %v",
				lb.service.name, vip.String(), endpoints, sourceVIP, n.Name(), err)
		}

		lbPolicylistMap[lb] = &policyLists{
			ilb: ilbPolicy,
		}

		for _, port := range ingressPorts {

			lbPolicylistMap[lb].elb, err = hcsshim.AddLoadBalancer(endpoints, false, sourceVIP, "", 0, uint16(port.TargetPort), uint16(port.PublishedPort))
			if err != nil {
				logrus.Errorf("Failed to add ELB policy for service %s (ip:%s target port:%v published port:%v) with endpoints %v using load balancer IP %s on network %s: %v",
					lb.service.name, vip.String(), uint16(port.TargetPort), uint16(port.PublishedPort), endpoints, sourceVIP, n.Name(), err)
				return
			}
		}
	}
}

func (n *network) rmLBBackend(ip, vip net.IP, lb *loadBalancer, ingressPorts []*PortConfig, rmService bool) {
	if system.GetOSVersion().Build > 16236 {
		if len(lb.backEnds) > 0 {
			//Reprogram HNS (actually VFP) with the existing backends.
			n.addLBBackend(ip, vip, lb, ingressPorts)
		} else {
			logrus.Debugf("No more backends for service %s (ip:%s).  Removing all policies", lb.service.name, lb.vip.String())

			if policyLists, ok := lbPolicylistMap[lb]; ok {
				if policyLists.ilb != nil {
					policyLists.ilb.Delete()
					policyLists.ilb = nil
				}

				if policyLists.elb != nil {
					policyLists.elb.Delete()
					policyLists.elb = nil
				}
				delete(lbPolicylistMap, lb)

			} else {
				logrus.Errorf("Failed to find policies for service %s (%s)", lb.service.name, lb.vip.String())
			}
		}
	}
}

func (sb *sandbox) populateLoadbalancers(ep *endpoint) {
}

func arrangeIngressFilterRule() {
}
