-- name: CreatePushSubscription :exec
INSERT INTO webpush_subscriptions(
	user_id,
	created_at,
	updated_at,
	expiration,
	subscription
) VALUES (
	@user_id,
	NOW(),
	NOW(),
	sqlc.narg('expiration'),
	@subscription
);

-- name: UpdatePushSubscription :execrows
UPDATE webpush_subscriptions
SET
	updated_at = NOW(),
	subscription = @new_subscription
WHERE
	user_id = @user_id AND
	subscription = @old_subscription;

	-- name: DeletePushSubscription :execrows
DELETE FROM webpush_subscriptions WHERE user_id = @user_id AND subscription = @subscription;

-- name: GetPushSubscriptions :many
SELECT id, user_id, subscription FROM webpush_subscriptions
WHERE
	expiration IS NULL OR expiration > NOW();

-- name: DeletePushSubscriptionByID :exec
DELETE FROM webpush_subscriptions WHERE id = @id;

-- name: DeletePushSubscriptionBySub :exec
DELETE FROM webpush_subscriptions WHERE subscription = @subscription;

-- name: DeleteExpiredPushSubscriptions :execrows
DELETE FROM webpush_subscriptions WHERE expiration IS NOT NULL AND expiration < NOW();

-- name: GetSubscriptionsSubscribed :many
SELECT DISTINCT
	wps.id, wps.user_id, wps.subscription
FROM webpush_subscriptions wps
LEFT OUTER JOIN talkgroup_notification_subscriptions tgns ON (wps.user_id = tgns.user_id)
LEFT OUTER JOIN system_notification_subscriptions sns ON (wps.user_id = sns.user_id)
WHERE
	(tgns.system_id = @system_id AND tgns.tgid = @tgid) OR
	(sns.system_id = @system_id);

-- name: SubscribeTalkgroups :exec
INSERT INTO talkgroup_notification_subscriptions (user_id, system_id, tgid) VALUES (
	@user_id, UNNEST(@system_ids::INT4[]), UNNEST(tgids::INT4[])
) ON CONFLICT DO NOTHING;

-- name: SubscribeSystem :exec
INSERT INTO system_notification_subscriptions (user_id, system_id) VALUES (@user_id, @system_id)
ON CONFLICT DO NOTHING;

-- name: UnsubscribeTalkgroups :exec
-- DELETE FROM talkgroup_notification_subscriptions WHERE user_id = @user_id AND (system_id, tgid) = ANY(@tgs);

-- name: UnsubscribeAllTalkgroups :exec
DELETE FROM talkgroup_notification_subscriptions WHERE user_id = @user_id;

-- name: UnsubscribeSystem :exec
DELETE FROM system_notification_subscriptions WHERE user_id = @user_id;
