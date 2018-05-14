package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/ghodss/yaml"
)

type role string

const (
	roleMaster  = "master"
	roleInfra   = "infra"
	roleCompute = "compute"
)

type awsHost struct {
	roles           []role
	instanceID      *string
	publicIPAddress *string
}

func newAWSHost(instanceID *string) *awsHost {
	return &awsHost{
		instanceID: instanceID,
	}
}

func (h *awsHost) addRole(role role) {
	h.roles = append(h.roles, role)
}

func (h *awsHost) hasRole(role role) bool {
	for _, hostRole := range h.roles {
		if hostRole == role {
			return true
		}
	}
	return false
}

type awsCluster struct {
	hosts []*awsHost
}

func newAWSCluster(instances []*ec2.Instance, masterCount, nodeCount int64) (*awsCluster, error) {
	if nodeCount < masterCount {
		return nil, fmt.Errorf("number of nodes is less than the number of masters")
	}

	hosts := make([]*awsHost, nodeCount)
	for i, instance := range instances {
		hosts[i] = newAWSHost(instance.InstanceId)
		if i < int(masterCount) {
			hosts[i].addRole(roleMaster)
		} else if i == int(masterCount) { // fix at 1 infra node for now
			hosts[i].addRole(roleInfra)
			hosts[i].addRole(roleCompute)
		} else {
			hosts[i].addRole(roleCompute)
		}
	}
	return &awsCluster{hosts: hosts}, nil
}

func (c *awsCluster) getHostByInstanceID(instanceID *string) (*awsHost, error) {
	for _, host := range c.hosts {
		if *host.instanceID == *instanceID {
			return host, nil
		}
	}
	return nil, fmt.Errorf("instanceID not found in cluster")
}

func (c *awsCluster) getHostsByRole(role role) []*awsHost {
	hosts := []*awsHost{}
	for _, host := range c.hosts {
		if host.hasRole(role) {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func (c *awsCluster) getInstanceIDs() []*string {
	instanceIDs := make([]*string, len(c.hosts))
	for i, host := range c.hosts {
		instanceIDs[i] = host.instanceID
	}
	return instanceIDs
}

type Node struct {
	IP      string
	IsInfra bool
}

type Inventory struct {
	Version     string
	Token       string
	ClusterName string
	Etcd        []Node
	Masters     []Node
	Nodes       []Node
}

type Config struct {
	MasterCount  int64  `yaml:"masterCount"`
	NodeCount    int64  `yaml:"nodeCount"`
	ClusterName  string `yaml:"clusterName"`
	Version      string `yaml:"version"`
	Token        string `yaml:"token"`
	ImageID      string `yaml:"imageID"`
	InstanceType string `yaml:"instanceType"`
	SubnetID     string `yaml:"subnetID"`
	KeyName      string `yaml:"keyName"`
}

func main() {
	configFile, err := ioutil.ReadFile("aws-launcher-config")
	if err != nil {
		log.Println("Could not open config file", err)
		return
	}

	var config Config
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Println("Could not unmarshall config file", err)
		return
	}
	log.Println(config)
	os.Exit(0)

	log.Println("Requesting %d instances")
	svc := ec2.New(session.New(&aws.Config{Region: aws.String("us-east-1")}))
	reservation, err := svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String(config.ImageID),
		InstanceType: aws.String(config.InstanceType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(config.NodeCount),
		SubnetId:     aws.String(config.SubnetID),
		KeyName:      aws.String(config.KeyName),
	})
	if err != nil {
		log.Println("Could not create instances", err)
		return
	}

	cluster, err := newAWSCluster(reservation.Instances, config.MasterCount, config.NodeCount)
	if err != nil {
		log.Println("Could not create cluster", err)
		return
	}

	// Add tags
	for _, host := range cluster.hosts {
		var suffix string
		if host.hasRole(roleMaster) {
			suffix = roleMaster
		} else if host.hasRole(roleInfra) {
			suffix = roleInfra
		} else {
			suffix = roleCompute
		}
		_, err = svc.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{host.instanceID},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("kubernetes.io/cluster/" + config.ClusterName),
					Value: aws.String("true"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String("sjenning-" + suffix),
				},
			},
		})
		if err != nil {
			log.Printf("Could not create tags for instance %s: %v", *host.instanceID, err)
			return
		}
	}
	log.Println("Successfully tagged instances")

	input := &ec2.DescribeInstancesInput{
		InstanceIds: cluster.getInstanceIDs(),
	}

	log.Println("Waiting for instances to be running")
	svc.WaitUntilInstanceRunning(input)

	log.Println("Getting instance IP information")
	result, err := svc.DescribeInstances(input)
	if err != nil {
		log.Println("Could not get IP information", err)
		return
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			host, _ := cluster.getHostByInstanceID(instance.InstanceId)
			host.publicIPAddress = instance.PublicIpAddress
		}
	}

	// Write out inventory
	inventory := Inventory{
		Version:     config.Version,
		Token:       config.Token,
		ClusterName: config.ClusterName,
	}
	// only first master has etcd, no etcd clustering supported at this time
	firstMaster := cluster.getHostsByRole(roleMaster)[0]
	inventory.Etcd = []Node{{IP: *firstMaster.publicIPAddress}}
	for _, master := range cluster.getHostsByRole(roleMaster) {
		inventory.Masters = append(inventory.Masters, Node{IP: *master.publicIPAddress})
	}
	for _, node := range cluster.getHostsByRole(roleCompute) {
		inventory.Nodes = append(inventory.Nodes, Node{IP: *node.publicIPAddress, IsInfra: node.hasRole(roleInfra)})
	}

	file, err := os.Create("inventory")
	if err != nil {
		log.Println("Could not open output inventory file", err)
		return
	}

	template, _ := template.ParseFiles("inventory.template")
	template.Execute(file, inventory)
	file.Close()
}
