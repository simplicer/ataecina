package domain

// Subscription is something the owner can spend on a provider, as the
// owner configured it: an id, the provider, the source that reads it, the
// fidelity that source gives and, optionally, a declared plan.
type Subscription struct {
	id       SubscriptionID
	provider ProviderID
	source   SourceID
	fidelity Fidelity
	plan     Plan
}

// NewSubscription describes one configured subscription.
func NewSubscription(id SubscriptionID, provider ProviderID, source SourceID, fidelity Fidelity, plan Plan) Subscription {
	return Subscription{id: id, provider: provider, source: source, fidelity: fidelity, plan: plan}
}

// ID is the owner-chosen id.
func (s Subscription) ID() SubscriptionID { return s.id }

// Provider is the company behind the subscription.
func (s Subscription) Provider() ProviderID { return s.provider }

// Source is where its readings come from.
func (s Subscription) Source() SourceID { return s.source }

// Fidelity is what that source can offer.
func (s Subscription) Fidelity() Fidelity { return s.fidelity }

// Plan is the plan declared by the owner, if any.
func (s Subscription) Plan() Plan { return s.plan }
