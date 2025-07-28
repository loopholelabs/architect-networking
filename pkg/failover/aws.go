package failover

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	logging "github.com/loopholelabs/logging/types"
)

// AWSClient handles AWS operations for ENI IP ownership detection
type AWSClient struct {
	EC2Client    *ec2.Client
	imdsClient   *imds.Client
	instanceID   string
	instanceMeta *imds.GetInstanceIdentityDocumentOutput
	logger       logging.Logger
}

// NewAWSClient creates a new AWS client with EC2 and IMDS capabilities
func NewAWSClient(ctx context.Context, logger logging.Logger) (*AWSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	EC2Client := ec2.NewFromConfig(cfg)
	imdsClient := imds.NewFromConfig(cfg)

	// Get instance metadata
	instanceDoc, err := imdsClient.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get instance identity document: %w", err)
	}

	return &AWSClient{
		EC2Client:    EC2Client,
		imdsClient:   imdsClient,
		instanceID:   instanceDoc.InstanceID,
		instanceMeta: instanceDoc,
		logger:       logger,
	}, nil
}

// GetInstanceID returns the current instance ID
func (a *AWSClient) GetInstanceID() string {
	return a.instanceID
}

// CheckENIOwnership checks if the current instance owns the given ENI IP address
func (a *AWSClient) CheckENIOwnership(ctx context.Context, eniIP string) (bool, error) {
	// Parse the IP to ensure it's valid
	ip := net.ParseIP(eniIP)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", eniIP)
	}

	// Describe network interfaces to find the one with this IP
	// Use addresses.private-ip-address to find both primary and secondary IPs
	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   &[]string{"addresses.private-ip-address"}[0],
				Values: []string{eniIP},
			},
		},
	}

	result, err := a.EC2Client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return false, fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	// Debug: log what we found
	a.logger.Debug().
		Str("eni_ip", eniIP).
		Int("interfaces_found", len(result.NetworkInterfaces)).
		Str("instance_id", a.instanceID).
		Msg("ENI ownership check details")

	// Check if any of the network interfaces are attached to this instance
	for _, nic := range result.NetworkInterfaces {
		var attachedInstance string
		if nic.Attachment != nil && nic.Attachment.InstanceId != nil {
			attachedInstance = *nic.Attachment.InstanceId
		}

		a.logger.Debug().
			Str("eni_id", *nic.NetworkInterfaceId).
			Str("attached_instance", attachedInstance).
			Str("our_instance", a.instanceID).
			Bool("is_attached", nic.Attachment != nil).
			Msg("Checking ENI attachment")

		if nic.Attachment != nil && nic.Attachment.InstanceId != nil {
			if *nic.Attachment.InstanceId == a.instanceID {
				a.logger.Info().
					Str("eni_id", *nic.NetworkInterfaceId).
					Str("eni_ip", eniIP).
					Msg("Found matching ENI attached to this instance")
				return true, nil
			}
		}
	}

	a.logger.Debug().
		Str("eni_ip", eniIP).
		Str("instance_id", a.instanceID).
		Msg("No matching ENI found attached to this instance")

	return false, nil
}

// GetENIOwner returns the instance ID that owns the given ENI IP address
func (a *AWSClient) GetENIOwner(ctx context.Context, eniIP string) (string, error) {
	// Parse the IP to ensure it's valid
	ip := net.ParseIP(eniIP)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", eniIP)
	}

	// Describe network interfaces to find the one with this IP
	// Use addresses.private-ip-address to find both primary and secondary IPs
	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   &[]string{"addresses.private-ip-address"}[0],
				Values: []string{eniIP},
			},
		},
	}

	result, err := a.EC2Client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	// Find the instance that owns this ENI
	for _, nic := range result.NetworkInterfaces {
		if nic.Attachment != nil && nic.Attachment.InstanceId != nil {
			return *nic.Attachment.InstanceId, nil
		}
	}

	return "", fmt.Errorf("no instance found owning ENI IP %s", eniIP)
}

// WaitForENIOwnership polls until the current instance owns the ENI IP
// This is useful during failover scenarios
func (a *AWSClient) WaitForENIOwnership(ctx context.Context, eniIP string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			owns, err := a.CheckENIOwnership(ctx, eniIP)
			if err != nil {
				return err
			}
			if owns {
				return nil
			}
			// TODO: Add exponential backoff or configurable polling interval
		}
	}
}

// GetENIFloatingIPs returns all secondary private IPs (floating IPs) for the given ENI
func (a *AWSClient) GetENIFloatingIPs(ctx context.Context, eniID string) ([]string, error) {
	input := &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []string{eniID},
	}

	result, err := a.EC2Client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe network interface %s: %w", eniID, err)
	}

	if len(result.NetworkInterfaces) == 0 {
		return nil, fmt.Errorf("network interface %s not found", eniID)
	}

	eni := result.NetworkInterfaces[0]
	var floatingIPs []string

	// Collect all secondary private IPs (floating IPs)
	for _, privateIP := range eni.PrivateIpAddresses {
		// Skip the primary IP
		if privateIP.Primary != nil && *privateIP.Primary {
			continue
		}
		if privateIP.PrivateIpAddress != nil {
			floatingIPs = append(floatingIPs, *privateIP.PrivateIpAddress)
		}
	}

	return floatingIPs, nil
}

// GetENIByIP returns the ENI ID that owns the given IP address
func (a *AWSClient) GetENIByIP(ctx context.Context, ip string) (string, error) {
	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("addresses.private-ip-address"),
				Values: []string{ip},
			},
		},
	}

	result, err := a.EC2Client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	if len(result.NetworkInterfaces) == 0 {
		return "", fmt.Errorf("no ENI found with IP %s", ip)
	}

	return *result.NetworkInterfaces[0].NetworkInterfaceId, nil
}

// FilterFloatingIPs filters IPs to only include those matching the pattern (e.g., x.x.x.20 onwards)
func (a *AWSClient) FilterFloatingIPs(ips []string, baseIP string) []string {
	// Parse the base IP to determine the pattern
	parts := strings.Split(baseIP, ".")
	if len(parts) != 4 {
		return ips // Return all if we can't parse
	}

	var filtered []string
	for _, ip := range ips {
		ipParts := strings.Split(ip, ".")
		if len(ipParts) != 4 {
			continue
		}

		// Check if first 3 octets match
		if ipParts[0] == parts[0] && ipParts[1] == parts[1] && ipParts[2] == parts[2] {
			// Parse last octet
			var lastOctet int
			if _, err := fmt.Sscanf(ipParts[3], "%d", &lastOctet); err == nil {
				// Check if it's 20 or higher
				if lastOctet >= 20 {
					filtered = append(filtered, ip)
				}
			}
		}
	}

	return filtered
}

// ReassignFloatingIPs moves floating IPs from source ENI to destination ENI
func (a *AWSClient) ReassignFloatingIPs(ctx context.Context, sourceENI, destENI string, ips []string) error {
	if len(ips) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(ips)*2) // Buffer for unassign and assign operations

	// Unassign IPs from source ENI in parallel
	for _, ip := range ips {
		wg.Add(1)
		go func(ipAddr string) {
			defer wg.Done()

			unassignInput := &ec2.UnassignPrivateIpAddressesInput{
				NetworkInterfaceId: aws.String(sourceENI),
				PrivateIpAddresses: []string{ipAddr},
			}

			_, err := a.EC2Client.UnassignPrivateIpAddresses(ctx, unassignInput)
			if err != nil {
				errCh <- fmt.Errorf("failed to unassign IP %s from ENI %s: %w", ipAddr, sourceENI, err)
				return
			}
		}(ip)
	}

	// Wait for all unassign operations to complete
	wg.Wait()

	// Assign IPs to destination ENI in parallel
	for _, ip := range ips {
		wg.Add(1)
		go func(ipAddr string) {
			defer wg.Done()

			assignInput := &ec2.AssignPrivateIpAddressesInput{
				NetworkInterfaceId: aws.String(destENI),
				PrivateIpAddresses: []string{ipAddr},
			}

			_, err := a.EC2Client.AssignPrivateIpAddresses(ctx, assignInput)
			if err != nil {
				errCh <- fmt.Errorf("failed to assign IP %s to ENI %s: %w", ipAddr, destENI, err)
				return
			}
		}(ip)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("floating IP reassignment errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// UpdateRouteTables updates all route tables to point to the new ENI
func (a *AWSClient) UpdateRouteTables(ctx context.Context, destinationCIDR, newENI string) error {
	a.logger.Info().
		Str("destination_cidr", destinationCIDR).
		Str("new_eni", newENI).
		Msg("Starting route table update")

	// First, find all route tables with routes to the destination CIDR
	describeInput := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("route.destination-cidr-block"),
				Values: []string{destinationCIDR},
			},
		},
	}

	a.logger.Debug().
		Str("filter", "route.destination-cidr-block").
		Str("value", destinationCIDR).
		Msg("Searching for route tables with filter")

	routeTables, err := a.EC2Client.DescribeRouteTables(ctx, describeInput)
	if err != nil {
		return fmt.Errorf("failed to describe route tables: %w", err)
	}

	a.logger.Info().
		Int("route_tables_found", len(routeTables.RouteTables)).
		Str("destination_cidr", destinationCIDR).
		Msg("Route tables search result")

	// Let's also search without filters to see what route tables exist
	if len(routeTables.RouteTables) == 0 {
		a.logger.Warn().Msg("No route tables found with filter, searching all route tables to debug")

		allRouteTables, err := a.EC2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{})
		if err != nil {
			a.logger.Error().Err(err).Msg("Failed to describe all route tables")
		} else {
			a.logger.Info().Int("total_route_tables", len(allRouteTables.RouteTables)).Msg("Total route tables in VPC")

			// Log details about routes in each table
			for _, rt := range allRouteTables.RouteTables {
				rtID := "unknown"
				if rt.RouteTableId != nil {
					rtID = *rt.RouteTableId
				}

				a.logger.Debug().
					Str("route_table_id", rtID).
					Int("route_count", len(rt.Routes)).
					Msg("Route table details")

				for i, route := range rt.Routes {
					destCIDR := "none"
					if route.DestinationCidrBlock != nil {
						destCIDR = *route.DestinationCidrBlock
					}

					target := "none"
					switch {
					case route.NetworkInterfaceId != nil:
						target = "eni:" + *route.NetworkInterfaceId
					case route.GatewayId != nil:
						target = "gw:" + *route.GatewayId
					case route.InstanceId != nil:
						target = "instance:" + *route.InstanceId
					}

					a.logger.Debug().
						Str("route_table_id", rtID).
						Int("route_index", i).
						Str("destination_cidr", destCIDR).
						Str("target", target).
						Msg("Route details")
				}
			}
		}

		return fmt.Errorf("no route tables found with routes to %s", destinationCIDR)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(routeTables.RouteTables))

	// Update routes in parallel
	for _, rt := range routeTables.RouteTables {
		wg.Add(1)
		go func(routeTableID string) {
			defer wg.Done()

			replaceInput := &ec2.ReplaceRouteInput{
				RouteTableId:         aws.String(routeTableID),
				DestinationCidrBlock: aws.String(destinationCIDR),
				NetworkInterfaceId:   aws.String(newENI),
			}

			_, err := a.EC2Client.ReplaceRoute(ctx, replaceInput)
			if err != nil {
				errCh <- fmt.Errorf("failed to update route in table %s: %w", routeTableID, err)
				return
			}
		}(*rt.RouteTableId)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("route table update errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// MoveEIPToENI moves an Elastic IP from its current ENI to a new ENI
func (a *AWSClient) MoveEIPToENI(ctx context.Context, privateIP, newENI string) error {
	a.logger.Info().
		Str("private_ip", privateIP).
		Str("new_eni", newENI).
		Msg("Starting EIP move")

	// First, find the EIP associated with this private IP
	addresses, err := a.EC2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("private-ip-address"),
				Values: []string{privateIP},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to find EIP for private IP %s: %w", privateIP, err)
	}

	if len(addresses.Addresses) == 0 {
		a.logger.Info().
			Str("private_ip", privateIP).
			Msg("No EIP associated with private IP, skipping EIP move")
		return nil
	}

	address := addresses.Addresses[0]
	allocationID := *address.AllocationId

	a.logger.Info().
		Str("allocation_id", allocationID).
		Str("public_ip", *address.PublicIp).
		Str("private_ip", privateIP).
		Str("new_eni", newENI).
		Msg("Moving EIP to new ENI")

	// Disassociate from current ENI (if associated)
	if address.AssociationId != nil {
		a.logger.Debug().
			Str("association_id", *address.AssociationId).
			Msg("Disassociating EIP from current ENI")

		_, err = a.EC2Client.DisassociateAddress(ctx, &ec2.DisassociateAddressInput{
			AssociationId: address.AssociationId,
		})
		if err != nil {
			return fmt.Errorf("failed to disassociate EIP %s: %w", allocationID, err)
		}
	}

	// Associate with new ENI
	a.logger.Debug().
		Str("allocation_id", allocationID).
		Str("new_eni", newENI).
		Str("private_ip", privateIP).
		Msg("Associating EIP with new ENI")

	_, err = a.EC2Client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
		AllocationId:       aws.String(allocationID),
		NetworkInterfaceId: aws.String(newENI),
		PrivateIpAddress:   aws.String(privateIP),
		AllowReassociation: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to associate EIP %s to ENI %s: %w", allocationID, newENI, err)
	}

	a.logger.Info().
		Str("allocation_id", allocationID).
		Str("public_ip", *address.PublicIp).
		Str("private_ip", privateIP).
		Str("new_eni", newENI).
		Msg("Successfully moved EIP")

	return nil
}

// TakeOverENI assigns the ENI IP to the current instance by moving it from another ENI
func (a *AWSClient) TakeOverENI(ctx context.Context, eniIP string) error {
	// First, find which ENI currently owns this IP
	currentENI, err := a.GetENIByIP(ctx, eniIP)
	if err != nil {
		return fmt.Errorf("failed to find current owner of IP %s: %w", eniIP, err)
	}

	// Get the ENI attached to this instance
	myENIResult, err := a.EC2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{a.instanceID},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to find ENI for instance %s: %w", a.instanceID, err)
	}

	if len(myENIResult.NetworkInterfaces) == 0 {
		return fmt.Errorf("no ENI found for instance %s", a.instanceID)
	}

	myENI := *myENIResult.NetworkInterfaces[0].NetworkInterfaceId

	// If we already own it, nothing to do
	if currentENI == myENI {
		return nil
	}

	// Unassign from current owner
	_, err = a.EC2Client.UnassignPrivateIpAddresses(ctx, &ec2.UnassignPrivateIpAddressesInput{
		NetworkInterfaceId: aws.String(currentENI),
		PrivateIpAddresses: []string{eniIP},
	})
	if err != nil {
		return fmt.Errorf("failed to unassign IP %s from ENI %s: %w", eniIP, currentENI, err)
	}

	// Assign to our ENI
	a.logger.Info().
		Str("eni_ip", eniIP).
		Str("target_eni", myENI).
		Msg("Assigning private IP to our ENI")

	_, err = a.EC2Client.AssignPrivateIpAddresses(ctx, &ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId: aws.String(myENI),
		PrivateIpAddresses: []string{eniIP},
	})
	if err != nil {
		return fmt.Errorf("failed to assign IP %s to ENI %s: %w", eniIP, myENI, err)
	}

	// Verify the IP was actually moved by checking ownership again
	a.logger.Debug().
		Str("eni_ip", eniIP).
		Str("expected_eni", myENI).
		Msg("Verifying IP move was successful")

	actualENI, err := a.GetENIByIP(ctx, eniIP)
	if err != nil {
		return fmt.Errorf("failed to verify IP move: %w", err)
	}

	if actualENI != myENI {
		return fmt.Errorf("IP move verification failed: expected ENI %s but IP %s is still on ENI %s", myENI, eniIP, actualENI)
	}

	a.logger.Info().
		Str("eni_ip", eniIP).
		Str("new_eni", myENI).
		Msg("Private IP move verified successful")

	// Move any associated EIP to our ENI
	err = a.MoveEIPToENI(ctx, eniIP, myENI)
	if err != nil {
		a.logger.Error().Err(err).Str("eni_ip", eniIP).Str("my_eni", myENI).Msg("Failed to move EIP, but private IP was moved successfully")
		// Don't return error here since the private IP move succeeded - EIP move is supplementary
	}

	return nil
}
