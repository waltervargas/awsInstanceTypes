package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/pprof"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type InstanceTypeProvider struct {
	excludeList       []string
	excludeListRegexp *regexp.Regexp
	ec2api            ec2iface.EC2API
}

func NewInstanceTypeProvider(ec2api ec2iface.EC2API, excludeList []string) *InstanceTypeProvider {
	// prepare the regex to apply excludelist
	var excludeListRegexp *regexp.Regexp
	if len(excludeList) > 0 {
		s := fmt.Sprintf("^(%s)$", strings.Join(excludeList, "|"))
		excludeListRegexp = regexp.MustCompile(s)
	}
	return &InstanceTypeProvider{
		ec2api:            ec2api,
		excludeListRegexp: excludeListRegexp,
		excludeList:       excludeList,
	}
}

func (p *InstanceTypeProvider) instanceTypeInExcludeList(instanceType string) bool {
	if p.excludeListRegexp == nil {
		return true
	}

	return p.excludeListRegexp.MatchString(instanceType)
}

func (p *InstanceTypeProvider) getInstanceTypes(ctx context.Context) ([]*ec2.InstanceTypeInfo, error) {
	instanceTypes := []*ec2.InstanceTypeInfo{}

	err := p.ec2api.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []*string{aws.String("hvm")},
			},
		},
	}, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		for _, instanceType := range page.InstanceTypes {
			if p.instanceTypeInExcludeList(*instanceType.InstanceType) {
				continue
			}

			instanceTypes = append(instanceTypes, instanceType)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("unable to fetch instance types")
	}
	return instanceTypes, nil
}

var (
	awsSession *session.Session
	ec2api     *ec2.EC2
)

func init() {
	awsSession = session.Must(session.NewSession())
	ec2api = ec2.New(awsSession)
}

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	excludeList := []string{
		"a1.metal",
		"a1.medium",
		"a1.large",
		"a1.xlarge",
		"a1.2xlarge",
		"a1.4xlarge",
	}

	instanceTypeProvider := NewInstanceTypeProvider(ec2api, excludeList)
	instanceTypes, err := instanceTypeProvider.getInstanceTypes(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(len(instanceTypes))
}
