Status: Closed
Labels: wayfinder:grilling
Parent: ../spec.md
Assignee: opencode

## Question

How should “minimal and fast” be made testable? Define representative workflows, cold-start and steady-state measurements, idle CPU/RAM targets, acceptable UI update overhead while downloads are active, and the validation method that will compare the selected architecture with rejected alternatives.

## Comments

### Resolution

Use broad MVP acceptance criteria rather than hard hardware-specific budgets: the app must start quickly to a usable UI, consume negligible CPU while idle, use modest idle memory, and remain responsive while three downloads are active. Validate the selected Wails implementation on a modest everyday Arch Linux laptop and repeat the same checks on Windows when that path is available.

The representative workflows are: launch to usable UI; paste and review a clipboard batch; and run three downloads while pausing, resuming, and restarting the app. Measure with lightweight OS process tools and app-level timestamps, supplemented by manual responsiveness checks. Do not add a dedicated profiling system to the MVP. Compare rejected frameworks through qualitative resource, maturity, and maintenance-risk analysis rather than building benchmark prototypes.
