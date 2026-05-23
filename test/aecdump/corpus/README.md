# .aecdump corpus

This directory holds real `.aecdump` recordings used by
`test/aecdump/replay_test.go`. They are committed via Git LFS — install
LFS before cloning if you intend to use the full set.

## Generation

The upstream tool `audioproc_f` (built from WebRTC's tree) writes
`.aecdump` files via `--dump-output`. For each scenario we record:

  * a brief description in `manifest.yaml`
  * the dump itself (`<scenario>.aecdump`)
  * the gopam-side pass/fail bounds in `../thresholds.yaml`

## Initial set (placeholders)

Replace these with real recordings as they become available:

  * `echo_easy.aecdump`           — single talker, low reverb, no near-end speech
  * `echo_doubletalk.aecdump`     — overlapping near/far speech, moderate room
  * `ns_noisy_room.aecdump`       — speech + café noise, no echo
  * `agc_quiet_to_loud.aecdump`   — single talker with a level step

## Licensing

Only commit dumps recorded from material we have the right to
redistribute (CC-BY / CC-0 corpora, internal-recorded speech with
contributor consent). When in doubt, leave it out and pull on demand.
