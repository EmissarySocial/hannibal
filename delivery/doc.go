// Package delivery turns one outbound ActivityPub activity into deliveries to the people it names.
//
// The work happens in two queue tasks. The fan-out task reads the activity's addressing, expands
// it into a list of recipients, removes duplicates and anyone the caller's rules exclude, and
// queues one send task for each recipient. The send task resolves one recipient's inbox, signs
// one HTTP request, and delivers it.
//
// The package holds protocol logic only. Everything that belongs to the application (who follows
// an actor, who is blocked, which hosts are failing, where email goes) is reached through small
// interfaces that the application implements and passes in. The package never adds a recipient
// to an activity: it delivers to whoever the activity already names.
package delivery
