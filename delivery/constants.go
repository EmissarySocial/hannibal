package delivery

import "time"

// FieldTo names a recipient that the activity lists in `to`
const FieldTo = "to"

// FieldCC names a recipient that the activity lists in `cc`
const FieldCC = "cc"

// FieldBTo names a recipient that the activity lists in `bto`
const FieldBTo = "bto"

// FieldBCC names a recipient that the activity lists in `bcc`
const FieldBCC = "bcc"

// FieldRelay names a recipient that the activity does not list: a relay that receives it because it is public
const FieldRelay = "relay"

// schemeMailto begins the URL of a recipient who receives the activity by email
const schemeMailto = "mailto:"

// TaskFanOut is the queue task that turns one activity into a send task for each recipient
const TaskFanOut = "Delivery:FanOut"

// TaskSend is the queue task that delivers one activity to one recipient's inbox
const TaskSend = "Delivery:Send"

// sendRetryMax is the number of times a delivery is retried. With sendBackoffMax, the retries
// double their wait for eleven attempts and then run daily, which covers just over seven days.
const sendRetryMax = 17

// sendBackoffMax is the longest wait between two attempts of one delivery
const sendBackoffMax = 24 * time.Hour
