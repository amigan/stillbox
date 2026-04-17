-- name: AddAlert :exec
INSERT INTO alerts (time, tgid, system_id, weight, score, orig_score, notified, metadata, context_window, url_tag)
VALUES
(
	@time,
	@tgid,
	@system_id,
	@weight,
	@score,
	@orig_score,
	@notified,
	@metadata,
	@context_window,
	@url_tag
);

-- name: GetAlertByURLTag :one
SELECT
	id,
	time,
	tgid,
	system_id,
	weight,
	score,
	orig_score,
	notified,
	metadata,
	context_window,
	url_tag
FROM alerts
WHERE url_tag = @url_tag
;

-- name: PruneAlerts :exec
DELETE FROM alerts WHERE time < @before;
