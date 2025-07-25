package did

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/internet_computer/candid/internal/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/parser"
)

// Method is a public method of a service.
type Method struct {
	// Name describes the method.
	Name string

	// Func is a function type describing its signature.
	Func *Func
	// ID is a reference to a type definition naming a function reference type.
	// It is NOT possible to have both a function type and a reference.
	ID *string
}

func (m Method) String() string {
	s := fmt.Sprintf("%s : ", m.Name)
	if id := m.ID; id != nil {
		return s + *id
	}
	return s + m.Func.String()
}

// Service can be used to declare the complete interface of a service. A service is a standalone actor on the platform
// that can communicate with other services via sending and receiving messages. Messages are sent to a service by
// invoking one of its methods, i.e., functions that the service provides.
//
// Example:
//
//	service : {
//		addUser : (name : text, age : nat8) -> (id : nat64);
//		userName : (id : nat64) -> (text) query;
//		userAge : (id : nat64) -> (nat8) query;
//		deleteUser : (id : nat64) -> () oneway;
//	}
type Service struct {
	// ID represents the optional name given to the service. This only serves as documentation.
	ID *string

	// Methods is the list of methods that the service provides.
	Methods []Method
	// MethodId is the reference to the name of a type definition for an actor reference type.
	// It is NOT possible to have both a list of methods and a reference.
	MethodId *string
}

func convertService(n *parser.Node) Service {
	var actor Service
	for _, n := range n.Children() {
		switch n.Name {
		case candid.Id.Name:
			id := n.Value()
			if actor.ID == nil {
				actor.ID = &id
				continue
			}
			actor.MethodId = &id
		case candid.TupType.Name:
		case candid.ActorType.Name:
			for _, n := range n.Children() {
				if n.Name == candid.CommentText.Name {
					continue
				}

				cs := n.Children()

				name := cs[0].Value()
				switch n := cs[len(cs)-1]; n.Name {
				case candid.FuncType.Name:
					f := convertFunc(n)
					actor.Methods = append(
						actor.Methods,
						Method{
							Name: name,
							Func: &f,
						},
					)
				case candid.Id.Name, candid.Text.Name:
					id := n.Value()
					actor.Methods = append(
						actor.Methods,
						Method{
							Name: name,
							ID:   &id,
						},
					)
				default:
					panic(n)
				}
			}
		default:
			panic(n)
		}
	}
	return actor
}

func (a Service) String() string {
	s := "service "
	if id := a.ID; id != nil {
		s += fmt.Sprintf("%s ", *id)
	}
	s += ": "
	if id := a.MethodId; id != nil {
		return s + *id
	}
	s += "{\n"
	for _, m := range a.Methods {
		s += fmt.Sprintf("  %s;\n", m.String())
	}
	return s + "}"
}
