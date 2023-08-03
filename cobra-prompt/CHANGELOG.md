# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2023-01-28

### Added

- `RunContext` - option to pass context into nested command execututions. ([#9](https://github.com/stromland/cobra-prompt/pull/9) by [@klowdo](https://github.com/klowdo))

## [0.4.0] - 2022-10-04

### Added

- `SuggestionFilter` to `CobraPrompt`. Function to decide which suggestions that should be presentet to the user. Overrides the current filter from go-prompt. ([#8](https://github.com/stromland/cobra-prompt/pull/8) by [@klowdo](https://github.com/klowdo))

## [0.3.0] - 2022-04-25

### Added

- `InArgsParser` to `CobraPrompt`. This makes it possible to decide how arguments should be structured before passing them to Cobra. ([#7](https://github.com/stromland/cobra-prompt/pull/7) by [@klowdo](https://github.com/klowdo))
