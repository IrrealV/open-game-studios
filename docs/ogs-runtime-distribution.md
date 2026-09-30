# OGS — arquitectura de distribución y runtime

Este documento consolida **cómo se distribuye OGS** y **con qué runtime se apoya**: su dependencia de una versión validada de Gentle Shell, la reutilización del Pi/Gentle Shell existente, el objetivo Linux/WSL primero y un ciclo de actualización que avisa y pide permiso antes de cambiar. Separa hechos observados de propuestas y pendientes.

**No es una autorización de ejecución.** No habilita instalar, configurar, clonar, actualizar ni ejecutar modelos o scripts. El backend del instalador (detección / plan / consentimiento / ejecución) está aceptado en alcance **fuente y fixture**, pero la instalación real o la reutilización compatible siguen **pendientes** (G7).

## Leyenda de estados

- **DECIDIDO** — elección humana vigente; no se reabre en este documento.
- **DOCUMENTADO U OBSERVADO** — afirmado en una fuente pública fijada o leído en este repositorio; no implica ejecución reproducida.
- **PROPUESTO** — recomendación pendiente de aprobación humana.
- **PENDIENTE** — sin resolver o sin verificar.

## 1. Las cuatro decisiones (DECIDIDO)

| # | Decisión | Consecuencia |
|---|---|---|
| 1 | OGS es un producto propio y depende de una **versión exacta y validada** de Gentle Shell, **no un fork**. | OGS no bifurca ni parchea Gentle Shell; se apoya en una combinación publicada y comprobada. |
| 2 | **Reutilizar el Pi/Gentle Shell existente** cuando sea compatible. | No se impone un entorno aislado ni una modalidad dual inicial; el usuario rechazó el entorno separado recomendado. |
| 3 | **Linux/WSL primero**; Windows nativo y macOS después. | Solo Linux/WSL es el objetivo inicial; lo demás no está validado. |
| 4 | **Avisar** de combinaciones validadas por OGS y **pedir permiso** antes de instalar. | No se sigue `latest` automáticamente; ninguna actualización ocurre sin aprobación. |

Estas son políticas de diseño. **No** seleccionan todavía versiones soportadas reales ni autorizan instalar o configurar nada.

## 2. Responsabilidad de cada capa

| Capa | Responsabilidad | Estado |
|---|---|---|
| Producto OGS | Dominio de estudio de videojuegos: crear, cambiar o reparar una mecánica de juego (Godot primero) con dirección coherente. Conserva el generador legacy y el paquete Pi. | DOCUMENTADO U OBSERVADO |
| Host Pi | Ejecuta sesiones, carga paquetes, skills, prompts, temas y extensiones; resuelve modelo, autenticación y ajustes. Es el host de ejecución, no el dueño del dominio. | DOCUMENTADO U OBSERVADO |
| Gentle Shell (`gentle-pi`) | Coordinación y delegación: ODD por defecto, subagentes, seguimiento de cambios, perfiles de agente y barrera de revisión. Aporta la forma de trabajo que OGS quiere conservar. | DOCUMENTADO U OBSERVADO |
| Runtime nativo de Gentle AI | Autoridad de revisión nativa (RDD) invocada por Gentle Shell mediante un binario local del paquete. Es una frontera de autoridad que OGS **no** redefine. | DOCUMENTADO U OBSERVADO |
| CCGS | Referencia de **dominio** de estudio (fases, roles, artefactos, compuertas de avance). No es una dependencia de runtime obligatoria. | DOCUMENTADO U OBSERVADO |

Aclaración de frontera: una copia nueva de este repositorio **no audita a fondo las interioridades** de la autoridad de revisión nativa de Gentle AI. La investigación interna de Gentle AI (`gentle-ai`) sigue **PENDIENTE**; no se afirma nada sobre su backend no inspeccionado.

## 3. Implementación actual frente al objetivo

### Actual (DOCUMENTADO U OBSERVADO)

- **Generador legacy en Go**: el wizard (`internal/cli/commands/wizard.go`) **genera** configuración y artefactos —config de workspace, `GAME-STUDIO.md`, perfil markdown, artefacto final JSON y validación smoke—. Marca la selección como «metadata only; not installed or started» y **no instala dependencias** de runtime hoy.
- **Backend del instalador aceptado**: detección de prerrequisitos, plan de huella (fingerprint), consentimiento explícito y ejecución acotada están implementados y aceptados en alcance **fuente y fixture**. La ejecución real de instalación o reutilización en un objetivo Linux/WSL autorizado (G7) sigue **pendiente** y no se observó.
- **Paquete Pi companion**: `package.json` declara `private: true` y publica `./skills/ogs-godot-change` y `./skills/ogs-core`; el trabajo se completa con `testdata/` (fixtures) y los checkers Node de `tests/`. `private: true` es un flag del paquete **npm**, no una decisión sobre la visibilidad del repositorio Git: el código fuente se prepara para un repositorio público. No bundlea ni instala Pi/Gentle Shell. Histórico: el loader público de Pi 0.85.1 validó el manifiesto con inventario metadata-only (`diagnostics: []`), sin body ni sesión.
- Sin instalaciones, clones, builds ni ejecución de modelos en este corte.

### Objetivo (PROPUESTO — no implementado)

Un instalador que reutilice el entorno compatible, proponga permisos de forma explícita y nunca cambie ajustes por deducción. Este documento describe su **comportamiento previsto**, no un producto terminado.

## 4. Rutas futuras del instalador (PROPUESTO)

| Caso | Comportamiento previsto |
|---|---|
| Compatible | Reutilizar el Pi/Gentle Shell existente sin reinstalar ni duplicar. |
| Faltan prerrequisitos | Proponer la instalación necesaria y **pedir permiso explícito** antes de tocar nada. |
| Incompatible o desconocido | Explicar la incompatibilidad y **detenerse**, u ofrecer un ajuste soportado y consentido. |

Límites que aplican a las tres rutas:

- Aprobar la **reutilización** no equivale a aprobar un **cambio de ajustes**.
- Sin downgrade ciego, reemplazo, desinstalación, creación de proveedores, copia de credenciales ni migración automática.
- Este documento **no** prescribe comandos reales de ejecución; un plan concreto y su consentimiento son pasos separados.

## 5. Matriz de compatibilidad — plantilla (PENDIENTE)

Plantilla para registrar una combinación **antes** de soportarla. Ninguna fila está validada hoy; completar la tabla no crea compatibilidad.

| Componente | Qué lo identifica | Cómo se registra | Fuente de evidencia |
|---|---|---|---|
| OGS (este repo) | Revisión del repositorio | Commit y rutas tocadas | Repo + commit |
| Host Pi | Pin **declarado** por la fuente vs identidad **instalada** (a observar) | Registrar la identidad/procedencia observada bajo alcance autorizado; la documentación solo aporta el pin declarado | `packages/coding-agent/docs/…` del commit fijado (**declarado**), no la instalación real |
| Gentle Shell (`gentle-pi`) | Paquete y versión | Versión de paquete | `package.json` y release publicada |
| Runtime nativo Gentle AI | Pin **declarado** por el paquete vs ejecutable **instalado** (a observar) | Registrar el ejecutable observado y su procedencia bajo alcance autorizado | `runtime/gentle-ai-binary.mjs` y pin del instalador (**declarado**), no la instalación real |
| Toolchain Go (solo build desde fuente) | Versión de Go | Versión mínima requerida | Documentación de instalación |
| Godot | Versión del motor | Versión exacta | Solo cuando un flujo de juego la requiera |

**Distinción clave: pin declarado ≠ instalación ≠ compatibilidad probada.** La documentación y los pins de fuente **no pueden demostrar** la procedencia del Pi instalado ni del ejecutable nativo, ni la compatibilidad de una combinación. Registrar por separado:

- (a) la **identidad/procedencia de la instalación observada**, bajo alcance autorizado;
- (b) el **pin declarado** por el paquete o la fuente;
- (c) la **evidencia de prueba de compatibilidad** real de una combinación concreta;
- la **configuración de modelo/autenticación** requiere su propia comprobación autorizada, aparte de (a)–(c).

Las observaciones y pruebas de (a) y (c), y la comprobación de modelo/auth, siguen **PENDIENTES**: no se realizan ni se autorizan aquí. La identidad instalada, la compatibilidad de API documentada y la verificación real de runtime son planos conceptuales separados; este documento no prescribe comandos ni inspecciona nada.

Criterios de uso de la matriz:

- **No inventar** una combinación soportada: cada celda requiere evidencia observada, no un rango de peerDependencies.
- Los **snapshots de investigación son evidencia de fuente, no el estado de releases actual**: las versiones anotadas en este estudio no prueban lo que npm publica hoy.
- Separar el **pin del binario nativo** de la **versión del lenguaje Go** que se exige para construir desde fuente: son dos decisiones distintas.
- Cuando la fuente use hashes (SHA-256, Go SumDB), describirlos como integridad de contenido, **no** como firmas verificadas.

## 6. Consecuencias del entorno compartido (DOCUMENTADO U OBSERVADO)

- Instalar un paquete en **scope de proyecto** (por ejemplo `pi install -l`, que escribe ajustes en `.pi/`) **no implica** que todo el estado de arranque o de perfil sea local.
- La resolución de agentes, definiciones, perfil, historial e hijos puede usar el **agent home** (cadenas de override documentadas hacia el home de agente), por lo que **otros usos de Pi pueden verse afectados**.
- El scope de instalación y el scope de perfil son capas distintas: conviene **detectar deriva de actualizaciones externas** antes de confiar en una capacidad.
- Límites a respetar: no se promete aislamiento total, rollback automático, cumplimiento universal ni compatibilidad garantizada a partir de un rango de peerDependencies.

## 7. Ciclo de actualización (PROPUESTO)

Secuencia recomendada, sin frecuencia operativa seleccionada:

1. Observar una **release publicada** (no `latest` ciego).
2. **Validar** flujos representativos de OGS sobre los objetivos soportados.
3. **Promover** una combinación soportada (registrada en la matriz).
4. **Notificar** al usuario con los efectos previstos.
5. **Aprobación explícita** y comprobaciones de sesión/estado antes de cambiar.

La **frecuencia y el mecanismo de sondeo no están seleccionados**. No se debe presentar una periodicidad como si fuera una decisión de producto ya tomada.

## 8. Preservación de estado y autoridad (DOCUMENTADO U OBSERVADO)

Contrato documentado por la fuente de Gentle Shell:

- Una vez que el runtime nativo fijado por el paquete **ha escrito autoridad de revisión** en un repositorio, un rollback **debe** preservar cada store y recibo nativos y **no debe** ejecutar un binario degradado contra ese repositorio.
- La recuperación válida es **avanzar hacia una versión compatible** o **detener la integración**; borrar autoridad o reinstalar un binario más viejo **no** es una ruta de rollback.
- Esto se limita al **contrato escrito de esa fuente**; OGS no redefine la autoridad nativa.

Implicaciones para OGS (DECIDIDO / PROPUESTO):

- El producto propio **no** reescribe ni migra autoridad de revisión; **no** cambia la política real de RDD (switch propiedad del usuario).
- Un enfoque de fork **no** elimina este riesgo: bifurcar referencias no cambia la semántica de autoridad nativa.
- El instalador objetivo trataría el downgrade del binario nativo como acción prohibida tras existir autoridad.

## 9. Pendientes técnicos y próximo corte (PENDIENTE)

- [ ] Verificar **versiones publicadas** actuales (Pi y Gentle Shell) contra los snapshots de este estudio.
- [ ] Verificar **APIs documentadas** y **comportamiento de estado compartido** (proyecto vs agent home) en una combinación concreta.
- [ ] Autorizar por separado **un** flujo real de cambio de mecánica en Godot, como corte acotado de validación.

Fuera de alcance del próximo corte:

- El estudio completo, sanitización de todo el flujo o suite total.
- Diagnóstico automático de NaN o piloto de API en vivo.
- Cualquier instalación o ejecución inferida de este documento: **no** hay consentimiento operativo implícito.

## 10. Referencias públicas portables

Referencias fijadas por repositorio, commit y ruta. El motivo de la **dependencia** frente al **fork** es conservar una base validada y no divergir del upstream.

| Proyecto | Origen público | Referencia (commit) | Rutas de evidencia |
|---|---|---|---|
| Gentle Shell | https://github.com/Gentleman-Programming/gentle-shell | `f2d9d073ffc2299eb501902753b42789f7cdc461` | `package.json`, `docs/readme-reference.md`, `docs/gentle-shell.md`, `runtime/gentle-ai-binary.mjs` |
| Pi | https://github.com/earendil-works/pi | tag `v0.86.0`, `ecac0a9c4edad3dac5d9f8b40e0c7db7a56471fc` | `packages/coding-agent/docs/packages.md`, `packages/coding-agent/docs/sdk.md` |
| CCGS | https://github.com/Donchitos/Claude-Code-Game-Studios | `984023ddac0d5e27624f2baacde6105e45de375f` | `.claude/docs/workflow-catalog.yaml` |
| Gentle AI | https://github.com/Gentleman-Programming/gentle-ai | `2336d09aefe2be7acaf243733368d4488f1652c9` | Auditoría interna PENDIENTE; sin afirmaciones de backend |

Notas de lectura:

- Los commits de CCGS, Gentle AI y Gentle Shell provienen de ramas predeterminadas remotas; **no** se afirman como releases estables.
- Pi se fija al tag estudiado; una versión coincidente no prueba identidad byte a byte con un binario instalado.
- Las versiones anotadas en este estudio son **pins declarados o históricos**; no certifican lo que npm publica hoy ni una instalación observada.

## 11. Cómo leer este documento

- Las cuatro decisiones del apartado 1 son humanas y no se reabren aquí.
- Todo lo marcado PROPUESTO o PENDIENTE requiere aprobación o verificación posterior.
- Para el estado actual del proyecto y su evidencia, ver [`docs/project-status.md`](project-status.md), [`docs/roadmap.md`](roadmap.md) y [`docs/architecture.md`](architecture.md).
- La adopción del estudio completo por capacidades está en [`ogs-adoption-matrix.md`](ogs-adoption-matrix.md).
- El primer corte acotado, con su plan de prueba propuesto, está en [`ogs-godot-speed-slice.md`](ogs-godot-speed-slice.md).

Siguiente paso: revisión humana del corte acotado de validación del apartado 9, con autorización separada.
