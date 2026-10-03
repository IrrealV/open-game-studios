extends Node3D

const SPEED := 3.0
const BOUNDS := 3.5
const COLLECTION_RADIUS := 0.6

var collected := false
var collection_events := 0

var player: Node3D
var collectible_body: Node3D
var message: Label

func _ready() -> void:
	player = $Player
	collectible_body = $Collectible/Body
	message = $UI/Message
	message.visible = false

func _physics_process(delta: float) -> void:
	var input_dir := Input.get_vector("move_left", "move_right", "move_forward", "move_back")
	
	if input_dir != Vector2.ZERO:
		player.position.x += input_dir.x * SPEED * delta
		player.position.z += input_dir.y * SPEED * delta
	
	player.position.x = clampf(player.position.x, -BOUNDS, BOUNDS)
	player.position.z = clampf(player.position.z, -BOUNDS, BOUNDS)
	
	if not collected:
		var dx := player.position.x - collectible_body.position.x
		var dz := player.position.z - collectible_body.position.z
		var distance := sqrt(dx * dx + dz * dz)
		
		if distance <= COLLECTION_RADIUS:
			collected = true
			collection_events += 1
			collectible_body.visible = false
			message.visible = true
			message.text = "Collected!"

func physics_step(direction: Vector2, delta_time: float) -> void:
	var input_dir := direction.normalized() if direction.length() > 0.0 else Vector2.ZERO
	
	if input_dir != Vector2.ZERO:
		player.position.x += input_dir.x * SPEED * delta_time
		player.position.z += input_dir.y * SPEED * delta_time
	
	player.position.x = clampf(player.position.x, -BOUNDS, BOUNDS)
	player.position.z = clampf(player.position.z, -BOUNDS, BOUNDS)
	
	if not collected:
		var dx := player.position.x - collectible_body.position.x
		var dz := player.position.z - collectible_body.position.z
		var distance := sqrt(dx * dx + dz * dz)
		
		if distance <= COLLECTION_RADIUS:
			collected = true
			collection_events += 1
			collectible_body.visible = false
			message.visible = true
			message.text = "Collected!"
