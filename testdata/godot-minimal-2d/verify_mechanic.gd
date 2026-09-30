extends SceneTree

const PlayerScene := preload("res://player.tscn")
const FRAME_COUNT := 12
const FIXED_DELTA := 1.0 / 60.0
const DISTANCE_TOLERANCE := 0.001

func _init() -> void:
	call_deferred("_verify")

func _verify() -> void:
	var parsed := _expected_speed_from_args()
	if not parsed.ok:
		_emit({"status": "error", "reason": parsed.reason})
		quit(2)
		return

	var expected_speed: float = parsed.value
	var player := PlayerScene.instantiate()
	get_root().add_child(player)
	var start_position: Vector2 = player.position

	for _frame in range(FRAME_COUNT):
		player.physics_step(Vector2.RIGHT, FIXED_DELTA)

	var measured_distance: float = player.position.x - start_position.x
	var expected_distance := expected_speed * FIXED_DELTA * FRAME_COUNT
	var error := absf(measured_distance - expected_distance)
	var measurement := {
		"status": "pass" if error <= DISTANCE_TOLERANCE else "fail",
		"expected_speed": expected_speed,
		"frames": FRAME_COUNT,
		"fixed_delta": FIXED_DELTA,
		"measured_distance": measured_distance,
		"expected_distance": expected_distance,
		"absolute_error": error,
		"tolerance": DISTANCE_TOLERANCE,
	}
	_emit(measurement)
	quit(0 if error <= DISTANCE_TOLERANCE else 1)

func _expected_speed_from_args() -> Dictionary:
	for argument in OS.get_cmdline_user_args():
		if not argument.begins_with("--expected-speed="):
			continue
		var raw_value := argument.trim_prefix("--expected-speed=")
		if not raw_value.is_valid_float():
			return {"ok": false, "reason": "expected speed must be a finite positive number"}
		var value := raw_value.to_float()
		if not is_finite(value) or value <= 0.0:
			return {"ok": false, "reason": "expected speed must be a finite positive number"}
		return {"ok": true, "value": value}
	return {"ok": false, "reason": "missing --expected-speed=<positive-number>"}

func _emit(measurement: Dictionary) -> void:
	print("OGS_MEASUREMENT " + JSON.stringify(measurement))
