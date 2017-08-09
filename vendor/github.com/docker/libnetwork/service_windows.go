package libnetwork

import (
	"net"

	"github.com/Microsoft/hcsshim"
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
	logrus.Debugf("Adding lb backend: %v %v portconfig:%v lb:%+v", ip, vip, ingressPorts, lb)

	var sourceVIP string
	for _, e := range n.Endpoints() {
		logrus.Debugf("****addLbBackend: endpointID: %v %v %+v*****", e.ID(), e.Name(), e)
		if e.Name() == "ingress-endpoint" {
			sourceVIP = e.Info().Iface().Address().IP.String()
		}
	}
	logrus.Debugf("****addLbBackend: sourceVIP: %v *****", sourceVIP)

	var endpoints []hcsshim.HNSEndpoint

	for eid := range lb.backEnds {
		//Call HNS to get back ID (GUID) corresponding to the endpoint.
		hnsEndpoint, err := hcsshim.GetHNSEndpointByName(eid)
		logrus.Debugf("****addLbBackend: GetHNSEndpointByName: %v %v %v*****", eid, hnsEndpoint, err)
		if err != nil {
			logrus.Debugf("****addLbBackend: Could not find HNS ID for endpoint %v. err: %v", eid, err)
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

	//ILBPolicyList, err := hcsshim.AddLoadBalancer(endpoints, true, sourceVIP, vip.String(), 0, uint16(port.TargetPort), uint16(port.PublishedPort))
	ilbPolicy, err := hcsshim.AddLoadBalancer(endpoints, true, sourceVIP, vip.String(), 0, 0, 0)
	if err != nil {
		//TODO: add more details.
		logrus.Debugf("****addLbBackend: Failed to ilb policy. err: %v", err)
	}

	lbPolicylistMap[lb] = &policyLists{
		ilb: ilbPolicy,
	}

	for _, port := range ingressPorts {

		logrus.Debugf("****addLbBackend: %v, %v, %v, %v  *****", endpoints, sourceVIP, vip, port)

		lbPolicylistMap[lb].elb, err = hcsshim.AddLoadBalancer(endpoints, false, sourceVIP, "", 0, uint16(port.TargetPort), uint16(port.PublishedPort))
		if err != nil {
			//TODO: add more details.
			logrus.Debugf("****addLbBackend: Failed to elb policy. err: %v", err)
			//TODO: how to clean up ILB policy ?????
			return
		}

		logrus.Debugf("****addLbBackend DONE: %v *****", lb)
	}

	//TODO: validate ILB only.  i.e. not ingress ports.
	//TODO: test multiple ports
	//TODO: Test multiple services.

}

func (n *network) rmLBBackend(ip, vip net.IP, lb *loadBalancer, ingressPorts []*PortConfig, rmService bool) {
	logrus.Debugf("rmLBBackend: Removing lb backend %v %v", ip, vip)

	if len(lb.backEnds) > 0 {
		//Reprogram VFP with the existing backends.
		logrus.Debugf("rmLBBackend: Reprogramming VFP for service %s (ID:%s VIP:%s).", lb.service.name, lb.service.id, lb.vip.String())
		n.addLBBackend(ip, vip, lb, ingressPorts)
	} else {
		logrus.Debugf("rmLBBackend: No more backends for %s (ID:%s VIP:%s), removing all policies", lb.service.name, lb.service.id, lb.vip.String())

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
			logrus.Debugf("rmLBBackend: XXXXXX Could not find policy list for %s %s %s", lb.service.name, lb.service.id, lb.vip.String())
			//TODO: handle this.
		}
	}
}

func (sb *sandbox) populateLoadbalancers(ep *endpoint) {
}

func arrangeIngressFilterRule() {
}
