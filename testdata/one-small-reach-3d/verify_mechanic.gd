extends SceneTree

const TOLERANCE := 5e-4
const MainScene := preload("res://main.tscn")

var checks_passed := 0
var checks_failed := 0

func _init() -> void:
	var expected_speed := 3.0
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--expected-speed="):
			expected_speed = float(arg.split("=")[1])
	
	print("OGS_CHECK Starting One Small Reach verification")
	
	# Check 1-4: Input map exists with WASD + arrows
	check_input_action("move_forward", [87, 4194320], "W/Up")
	check_input_action("move_back", [83, 4194322], "S/Down")
	check_input_action("move_left", [65, 4194319], "A/Left")
	check_input_action("move_right", [68, 4194321], "D/Right")
	
	# Load scene
	var main_instance: Node = MainScene.instantiate()
	root.add_child(main_instance)
	
	# Manually initialize references (scene not fully ready yet)
	main_instance._ready()
	
	# Check 5: Initial positions and state
	var player: Node3D = main_instance.player
	var collectible: Node3D = main_instance.get_node("Collectible")
	var collectible_body: Node3D = main_instance.collectible_body
	var message: Label = main_instance.message
	
	check_near(player.position.x, -2.0, "Player initial X")
	check_near(player.position.z, 0.0, "Player initial Z")
	check_bool(not main_instance.collected, "Initially not collected")
	check_bool(not message.visible, "Message initially hidden")
	check_bool(collectible_body.visible, "Collectible initially visible")
	
	# Check 6: Forward movement
	main_instance.physics_step(Vector2(0, -1), 1.0)  # Forward (negative Z)
	var moved_z := player.position.z
	check_near(moved_z, -expected_speed, "Forward movement speed")
	
	# Check 7: Normalized diagonal
	player.position = Vector3(-2, 0.8, 0)
	main_instance.physics_step(Vector2(1, -1), 1.0)  # Right + Forward
	var diag_dist := sqrt(player.position.x * player.position.x + player.position.z * player.position.z)
	var expected_diag := sqrt(2.0 * 2.0 + expected_speed * expected_speed)
	check_near(diag_dist, expected_diag, "Normalized diagonal")
	
	# Check 8: Immediate stop
	var pos_before := player.position
	main_instance.physics_step(Vector2.ZERO, 1.0)
	check_near(player.position.x, pos_before.x, "Stop X (no drift)")
	check_near(player.position.z, pos_before.z, "Stop Z (no drift)")
	
	# Check 9: Bounds clamp (both axes)
	player.position = Vector3(3.4, 0.8, 3.4)
	main_instance.physics_step(Vector2(1, -1), 1.0)  # Try to exceed bounds
	check_bool(player.position.x <= 3.5 and player.position.x >= -3.5, "Bounds X clamp")
	check_bool(player.position.z <= 3.5 and player.position.z >= -3.5, "Bounds Z clamp")
	
	# Check 10: Collection threshold bracket
	player.position = Vector3(2.0, 0.8, 0.59)  # Distance ~0.59 from collectible at (2, 0.8, 0)
	main_instance.physics_step(Vector2.ZERO, 0.0)
	check_bool(main_instance.collected, "Collect at 0.59 distance")
	check_bool(not collectible_body.visible, "Sphere hidden")
	check_bool(message.visible, "Message visible")
	
	# Reset for negative test
	main_instance.collected = false
	main_instance.collection_events = 0
	collectible_body.visible = true
	message.visible = false
	collectible.position = Vector3(2, 0.8, 0)  # Reset collectible parent position
	player.position = Vector3(2.0, 0.8, 0.61)  # Distance ~0.61 (should NOT collect)
	main_instance.physics_step(Vector2.ZERO, 0.0)
	check_bool(not main_instance.collected, "No collect at 0.61 distance")
	
	# Check 11: Once-only collection
	player.position = Vector3(2.0, 0.8, 0.0)  # Collect
	main_instance.physics_step(Vector2.ZERO, 0.0)
	var events_after_first: int = main_instance.collection_events
	player.position = Vector3(3.0, 0.8, 0.0)  # Move away
	main_instance.physics_step(Vector2.ZERO, 0.0)
	player.position = Vector3(2.0, 0.8, 0.0)  # Return
	main_instance.physics_step(Vector2.ZERO, 0.0)
	check_bool(main_instance.collection_events == events_after_first, "Once-only collection")
	
	# Check 12: Movement after collection
	var pos_before_move := player.position
	main_instance.physics_step(Vector2(1, 0), 0.5)
	check_bool(player.position.x > pos_before_move.x, "Movement continues after collection")
	
	print("OGS_MEASUREMENT checks_passed=%d checks_failed=%d" % [checks_passed, checks_failed])
	
	if checks_failed > 0:
		quit(1)
	else:
		print("OGS_CHECK All checks passed")
		quit(0)

func check_input_action(action_name: String, expected_keycodes: Array, label: String) -> void:
	if not InputMap.has_action(action_name):
		print("OGS_CHECK FAIL: Missing input action '%s'" % action_name)
		checks_failed += 1
		return
	
	var events := InputMap.action_get_events(action_name)
	var found_keycodes := []
	for event in events:
		var key_event := event as InputEventKey
		if key_event:
			found_keycodes.append(key_event.physical_keycode)
	
	var all_found := true
	for code in expected_keycodes:
		if not (code in found_keycodes):
			all_found = false
			break
	
	if all_found:
		print("OGS_CHECK PASS: Input action '%s' (%s)" % [action_name, label])
		checks_passed += 1
	else:
		print("OGS_CHECK FAIL: Input action '%s' missing keycodes (expected %s, found %s)" % [action_name, expected_keycodes, found_keycodes])
		checks_failed += 1

func check_near(actual: float, expected: float, label: String) -> void:
	if abs(actual - expected) <= TOLERANCE:
		print("OGS_CHECK PASS: %s (%.4f ~= %.4f)" % [label, actual, expected])
		checks_passed += 1
	else:
		print("OGS_CHECK FAIL: %s (%.4f != %.4f, diff %.4f)" % [label, actual, expected, abs(actual - expected)])
		checks_failed += 1

func check_bool(condition: bool, label: String) -> void:
	if condition:
		print("OGS_CHECK PASS: %s" % label)
		checks_passed += 1
	else:
		print("OGS_CHECK FAIL: %s" % label)
		checks_failed += 1
