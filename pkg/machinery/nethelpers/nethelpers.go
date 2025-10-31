// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nethelpers provides types and type wrappers to support network resources.
package nethelpers

//go:generate go tool github.com/dmarkham/enumer -type=ARPAllTargets,ARPValidate,AddressFlag,AddressSortAlgorithm,ADSelect,ADLACPActive,AutoHostnameKind,BondMode,BondXmitHashPolicy,ClientIdentifier,ConntrackState,DefaultAction,Duplex,Family,LACPRate,LinkFlag,LinkType,MatchOperator,NfTablesChainHook,NfTablesChainPriority,NfTablesVerdict,OperationalState,Port,PrimaryReselect,Protocol,RouteFlag,RouteProtocol,RouteType,RoutingTable,Scope,Status,VLANProtocol,WOLMode -linecomment -text
//go:generate go tool github.com/dmarkham/enumer -type=FailOverMAC -linecomment
