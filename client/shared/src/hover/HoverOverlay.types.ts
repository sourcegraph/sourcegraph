import { HoverMerged } from '@sourcegraph/client-api'
import { HoverOverlayProps as GenericHoverOverlayProps } from '@sourcegraph/codeintellify'

import { ActionItemAction } from '../actions/ActionItem'
import { FileSpec, RepoSpec, ResolvedRevisionSpec, RevisionSpec } from '../util/url'

export type HoverContext = RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec

export interface HoverOverlayBaseProps extends GenericHoverOverlayProps<HoverContext, HoverMerged, ActionItemAction> {}
