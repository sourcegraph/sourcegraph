import { isValidElement } from 'react'

type FirstArgument = Parameters<typeof isValidElement>[0]

export const getReactElements = (array: FirstArgument[]): JSX.Element[] => array.filter<JSX.Element>(isValidElement)
