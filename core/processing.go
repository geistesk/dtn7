package core

import (
	"log"
	"sync"

	"github.com/geistesk/dtn7/bundle"
	"github.com/geistesk/dtn7/cla"
)

// SendBundle transmits an outbounding bundle.
func (c *Core) SendBundle(bndl bundle.Bundle) {
	c.transmit(NewBundlePack(bndl))
}

// transmit starts the transmission of an outbounding bundle pack. Therefore
// the source's endpoint ID must be dtn:none or a member of this node.
func (c *Core) transmit(bp BundlePack) {
	log.Printf("Transmission of bundle requested: %v", bp.Bundle)

	c.idKeeper.update(bp.Bundle)

	bp.AddConstraint(DispatchPending)
	c.store.Push(bp)

	src := bp.Bundle.PrimaryBlock.SourceNode
	if src != bundle.DtnNone() && !c.HasEndpoint(src) {
		log.Printf(
			"Bundle's source %v is neither dtn:none nor an endpoint of this node", src)

		c.bundleDeletion(bp, NoInformation)
		return
	}

	c.dispatching(bp)
}

// receive handles received/incoming bundles.
func (c *Core) receive(bp BundlePack) {
	log.Printf("Received new bundle: %v", bp.Bundle)

	if KnowsBundle(c.store, bp) {
		log.Printf("Received bundle's ID is already known.")

		// bundleDeletion is _not_ called because this would delete the already
		// stored BundlePack.
		return
	}

	bp.AddConstraint(DispatchPending)
	c.store.Push(bp)

	if bp.Bundle.PrimaryBlock.BundleControlFlags.Has(bundle.StatusRequestReception) {
		c.SendStatusReport(bp, ReceivedBundle, NoInformation)
	}

	for i := len(bp.Bundle.CanonicalBlocks) - 1; i >= 0; i-- {
		var cb = bp.Bundle.CanonicalBlocks[i]

		if isKnownBlockType(cb.BlockType) {
			continue
		}

		log.Printf("Bundle's %v canonical block is unknown, type %d",
			bp.Bundle, cb.BlockType)

		if cb.BlockControlFlags.Has(bundle.StatusReportBlock) {
			log.Printf("Bundle's %v unknown canonical block requested reporting",
				bp.Bundle)

			c.SendStatusReport(bp, ReceivedBundle, BlockUnintelligible)
		}

		if cb.BlockControlFlags.Has(bundle.DeleteBundle) {
			log.Printf("Bundle's %v unknown canonical block requested bundle deletion",
				bp.Bundle)

			c.bundleDeletion(bp, BlockUnintelligible)
			return
		}

		if cb.BlockControlFlags.Has(bundle.RemoveBlock) {
			log.Printf("Bundle's %v unknown canonical block requested to be removed",
				bp.Bundle)

			bp.Bundle.CanonicalBlocks = append(
				bp.Bundle.CanonicalBlocks[:i], bp.Bundle.CanonicalBlocks[i+1:]...)
		}
	}

	c.dispatching(bp)
}

// dispatching handles the dispatching of received bundles.
func (c *Core) dispatching(bp BundlePack) {
	log.Printf("Dispatching bundle %v", bp.Bundle)

	if c.HasEndpoint(bp.Bundle.PrimaryBlock.Destination) {
		c.localDelivery(bp)
	} else {
		c.forward(bp)
	}
}

// forward forwards a bundle pack's bundle to another node.
func (c *Core) forward(bp BundlePack) {
	log.Printf("Bundle will be forwarded: %v", bp.Bundle)

	bp.AddConstraint(ForwardPending)
	bp.RemoveConstraint(DispatchPending)
	c.store.Push(bp)

	if hcBlock, err := bp.Bundle.ExtensionBlock(bundle.HopCountBlock); err == nil {
		hc := hcBlock.Data.(bundle.HopCount)
		hc.Increment()
		hcBlock.Data = hc

		log.Printf("Bundle %v contains an hop count block: %v", bp.Bundle, hc)

		if exceeded := hc.IsExceeded(); exceeded {
			log.Printf("Bundle contains an exceeded hop count block: %v", hc)

			c.bundleDeletion(bp, HopLimitExceeded)
			return
		}
	}

	if bp.Bundle.PrimaryBlock.IsLifetimeExceeded() {
		log.Printf("Bundle's primary block's lifetime is exceeded: %v",
			bp.Bundle.PrimaryBlock)

		c.bundleDeletion(bp, LifetimeExpired)
		return
	}

	if age, err := bp.UpdateBundleAge(); err == nil {
		if age >= bp.Bundle.PrimaryBlock.Lifetime {
			log.Printf("Bundle's lifetime is expired")

			c.bundleDeletion(bp, LifetimeExpired)
			return
		}
	}

	var nodes []cla.ConvergenceSender
	var deleteAfterwards = true

	// Try a direct delivery or consult the RoutingAlgorithm otherwise.
	nodes = c.senderForDestination(bp.Bundle.PrimaryBlock.Destination)
	if nodes == nil {
		nodes, deleteAfterwards = c.routing.SenderForBundle(bp)
	}

	if nodes == nil {
		// No nodes could be selected, the bundle will be contraindicated.
		c.bundleContraindicated(bp)
		return
	}

	var bundleSent = false

	var wg sync.WaitGroup
	var once sync.Once

	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node cla.ConvergenceSender) {
			log.Printf("Trying to deliver bundle %v to %v", bp.Bundle, node)

			if err := node.Send(*bp.Bundle); err != nil {
				log.Printf("Transmission of bundle %v failed to %v: %v",
					bp.Bundle, node, err)

				log.Printf("Restarting ConvergenceSender %v", node)
				node.Close()
				c.RemoveConvergenceSender(node)
				c.RegisterConvergenceSender(node)
			} else {
				log.Printf("Transmission of bundle %v succeeded to %v", bp.Bundle, node)

				once.Do(func() { bundleSent = true })
			}

			wg.Done()
		}(node)
	}

	wg.Wait()

	if bundleSent {
		if bp.Bundle.PrimaryBlock.BundleControlFlags.Has(bundle.StatusRequestForward) {
			c.SendStatusReport(bp, ForwardedBundle, NoInformation)
		}

		if deleteAfterwards {
			bp.PurgeConstraints()
			c.store.Push(bp)
		} else if c.inspectAllBundles && bp.Bundle.IsAdministrativeRecord() {
			c.bundleContraindicated(bp)
			c.checkAdministrativeRecord(bp)
		}
	} else {
		log.Printf("Failed to forward %v", bp.Bundle)
		c.bundleContraindicated(bp)
	}
}

// checkAdministrativeRecord checks administrative records. If this method
// returns false, an error occured.
func (c *Core) checkAdministrativeRecord(bp BundlePack) bool {
	if !bp.Bundle.IsAdministrativeRecord() {
		log.Printf("Bundle %v does not contain an administrative record", bp.Bundle)
		return false
	}

	canonicalAr, err := bp.Bundle.PayloadBlock()
	if err != nil {
		log.Printf("Bundle %v with an administrative record payload misses payload: %v",
			bp.Bundle, err)

		return false
	}

	ar, err := NewAdministrativeRecordFromCbor(canonicalAr.Data.([]byte))
	if err != nil {
		log.Printf("Bundle %v with an administrative record could not be parsed: %v",
			bp.Bundle, err)

		return false
	}

	log.Printf("Received bundle %v contains an administrative record: %v",
		bp.Bundle, ar)
	c.inspectStatusReport(ar)

	return true
}

func (c *Core) inspectStatusReport(ar AdministrativeRecord) {
	var status = ar.Content
	var sips = status.StatusInformations()

	if len(sips) == 0 {
		log.Printf("Administrative record %v contains no status information", ar)
		return
	}

	var bps = QueryFromStatusReport(c.store, status)
	if len(bps) != 1 {
		log.Printf("Status Report's (%v) bundle is unknown", status)
		return
	}
	var bp = bps[0]

	for _, sip := range sips {
		switch sip {
		case ReceivedBundle:
			log.Printf("Status Report %v indicates a received bundle", status)

		case ForwardedBundle:
			log.Printf("Status Report %v indicates a forwarded bundle", status)

		case DeliveredBundle:
			log.Printf("Status Report %v indicates a delivered bundle", status)

			bp.PurgeConstraints()
			c.store.Push(bp)

		case DeletedBundle:
			log.Printf("Status Report %v indicates a deleted bundle", status)

		default:
			log.Printf("Status Report %v has an unknown status information", status)
		}
	}
}

func (c *Core) localDelivery(bp BundlePack) {
	// TODO: check fragmentation

	log.Printf("Received delivered bundle: %v", bp.Bundle)

	if bp.Bundle.IsAdministrativeRecord() {
		if !c.checkAdministrativeRecord(bp) {
			c.bundleDeletion(bp, NoInformation)
			return
		}
	}

	for _, agent := range c.Agents {
		if agent.EndpointID() == bp.Bundle.PrimaryBlock.Destination {
			agent.Deliver(bp.Bundle)
		}
	}

	c.routing.NotifyIncoming(bp)

	if bp.Bundle.PrimaryBlock.BundleControlFlags.Has(bundle.StatusRequestDelivery) {
		c.SendStatusReport(bp, DeliveredBundle, NoInformation)
	}

	bp.PurgeConstraints()
	c.store.Push(bp)
}

func (c *Core) bundleContraindicated(bp BundlePack) {
	log.Printf("Bundle %v was marked for contraindication", bp.Bundle)

	bp.AddConstraint(Contraindicated)
	c.store.Push(bp)
}

func (c *Core) bundleDeletion(bp BundlePack, reason StatusReportReason) {
	if bp.Bundle.PrimaryBlock.BundleControlFlags.Has(bundle.StatusRequestDeletion) {
		c.SendStatusReport(bp, DeletedBundle, reason)
	}

	bp.PurgeConstraints()
	c.store.Push(bp)

	log.Printf("Bundle %v was marked for deletion", bp.Bundle)
}
