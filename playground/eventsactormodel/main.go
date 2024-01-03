package main

import (
	"github.com/davidroman0O/seigyo/playground/eventsactormodel/events"
)

type Something struct {
	Msg string
}

func main() {

	events.NewActor(
		events.ActorName(func() events.ActorAddressName {
			return "something"
		}),
		// should create an inbox for Something while pre-interest the actor to Something
		events.NewReceiver[Something](
			func(ctx events.ActorContext, message Something) error {
				return nil
			},
		),
		// events.ActorReceiver[Something](func(ctx events.ActorContext) func(receive Something) {
		// 	return func(receive Something) {

		// 	}
		// }),
	)

}
