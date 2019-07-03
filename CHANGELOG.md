# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [1.0.3] - 2019-07-03

### Changed

- Add support for getting summary metrics by default for virtual machine and datastore metrics. This help to avoid the intensive calls for performance metrics for thes two types.

## [1.0.2] - 2019-01-10

### Changed

- Separated user credentials into separate parameters (previously they had to be passed within the URL). This change is backward compatible.
- updated the infra-integrations-sdk version

## [1.0.0] - 2018-08-30

### Added

- Initial version: Includes Metrics and Inventory data
