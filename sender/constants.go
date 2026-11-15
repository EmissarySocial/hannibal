package sender

import "time"

// OutboxSendToAllRecipients is the name of the task that scans an
// outbound ActivityPub activity for recipients' inbox URLs,
// then queues additional tasks to send that activity to
// each recipient
const OutboxSendToAllRecipients = "Outbox:SendToAllRecipients"

// OutboxSendToSingleRecipient is the name of the task that sends
// an outbound ActivityPub activity to a single recipient.
const OutboxSendToSingleRecipient = "Outbox:SendToSingleRecipient"

// argumentQueuedAt names the task argument that holds when a delivery was first queued, in
// Unix seconds. It outlasts a reschedule, which the task's own counters do not.
const argumentQueuedAt = "queuedAt"

// minimumRequeueDelay is the shortest wait before a rescheduled delivery runs again
const minimumRequeueDelay = time.Second

// maximumRequeueDelay is the longest wait that a host can ask of a rescheduled delivery
const maximumRequeueDelay = time.Hour

// maximumDeliveryAge is how long a host may keep rescheduling one delivery with a rate limit
const maximumDeliveryAge = 7 * 24 * time.Hour
