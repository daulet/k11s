# Bugs

- [x] order of tab completions - sorted alphabetically?
- [x] shift tab to go back in suggestion
- [x] arrows up/down should work too?

# Features

- [x] improve list views: column headers, search (with "/" vim style and related keybindings), jump multiple nodes bindings
- [x] Add node listing and other namespace less resources. Navigation should correctly reflect that it is not namespace based 
- [x] things in the list need to be clickable, eg pod list should have a node in one of columns, clicking which should open node. clicking namespace. clicking owner resource should do the same
- [x] since things are constatnly refreshing they should flash temporarily (highlighted) if they changed. eg if pod status changed - flash it
- [x] Pod view: hitting enter in pod view should open pod view, escape to go back.
      1. It should have tabbed view: tabs for overview, each container, logs, events, and its yaml definition
      2. Overview should include Owner, labels, annotations, Phase, Conditions, IP, service account, node, node selector, tolerations, age
      3. Container view should contain image, command, status, restarts, last restart, restart reason, results from probes (startup/liveness/readiness), list of env var, ports, mounts
      4. Logs should have an option to filter by container if multiple containers
      5. All resources mentioned above need to be clickable eg clicking owner should open that resource, same for node
