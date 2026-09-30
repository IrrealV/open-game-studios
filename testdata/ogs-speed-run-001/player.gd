extends CharacterBody2D

@export var speed: float = 240.0

func _physics_process(delta: float) -> void:
	var direction := Input.get_vector("ui_left", "ui_right", "ui_up", "ui_down")
	physics_step(direction, delta)

func physics_step(direction: Vector2, delta: float) -> void:
	if direction.length_squared() > 0.0:
		velocity = direction.normalized() * speed
	else:
		velocity = Vector2.ZERO
	move_and_collide(velocity * delta)
