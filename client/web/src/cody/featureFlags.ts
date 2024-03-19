import { useFeatureFlag } from '../featureFlags/useFeatureFlag'

// TODO(sourcegraph:#60213) remove this flag after testing is complete
export const useIsCodyPaymentsTestingMode = (): boolean => {
    const [testingMode] = useFeatureFlag('cody-payments-testing-mode', false)

    return testingMode
}
