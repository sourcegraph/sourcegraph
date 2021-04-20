import classnames from 'classnames';
import React from 'react';
import { Form } from 'reactstrap';

import { Page } from '../../../components/Page';
import { PageTitle } from '../../../components/PageTitle';

import { InputField } from './components/form-field/FormField';
import { FormGroup } from './components/form-group/FormGroup';
import { FormRadioInput } from './components/form-radio-input/FormRadioInput';
import { FormColorPicker } from './components/form-series-input/FormSeriesInput';
import styles from './CreateinsightPage.module.scss'

interface CreateInsightPageProps {}

export const CreateInsightPage: React.FunctionComponent<CreateInsightPageProps> = () => (
        <Page className='col-8'>
            <PageTitle title='Create new code insight'/>

            <div className={styles.createInsightSubTitleContainer}>

                <h2>Create new code insight</h2>

                <p className='text-muted'>
                    Search-based code insights analyse your code based on any search query.
                    {' '}
                    <a href="https://docs.sourcegraph.com/code_monitoring/how-tos/starting_points"
                       target="_blank"
                       rel="noopener">
                        Learn more.
                    </a>
                </p>
            </div>

            <Form className={styles.createInsightForm}>

                <InputField
                    name='Name'
                    description='Chose a unique for your insights'
                    placeholder='Enter the unique name for your insight'
                    className={styles.createInsightFormField}/>

                <InputField
                    name='Title'
                    description='Shown as title for your insight'
                    placeholder='ex. Migration to React function components'
                    className={styles.createInsightFormField}/>

                <InputField
                    name='Repositories'
                    description='Create a list of repositories to run your search over. Separate them with comas.'
                    placeholder='Add or search for repositories'
                    className={styles.createInsightFormField}/>

                <FormGroup
                    name='Visibility'
                    description='This insigh will be visible only on your personal dashboard. It will not be show to other
                        users in your organisation.'
                    className={styles.createInsightFormField}>

                    <div className={styles.createInsightRadioGroupContent}>

                        <FormRadioInput
                            name='Personal'
                            description='only for you'
                            className={styles.createInsightRadio}/>
                        <FormRadioInput
                            name='Organization'
                            description='to all users in your organization'
                            className={styles.createInsightRadio}/>
                    </div>
                </FormGroup>

                <hr className={styles.createInsightSeparator}/>

                <FormGroup
                    name='Data series'
                    subtitle='Add any number of data series to your chart'>

                    <div className={styles.createInsightSeriesContent}>

                        <InputField
                            name='Name'
                            placeholder='ex. Function component'
                            description='Name shown in the legend and tooltip'
                            className={styles.createInsightFormFieldSeries}/>

                        <InputField
                            name='Query'
                            placeholder='ex. spatternType:regexp const\\s\\w+:\\s(React\\.)?FunctionComponent'
                            description='Do not include the repo: filter as it will be added automatically for the current repository'
                            className={styles.createInsightFormFieldSeries}/>

                        <FormGroup
                            name='Color'
                            className={styles.createInsightFormFieldSeries}>

                            <FormColorPicker/>
                        </FormGroup>

                        <button
                            type='button'
                            className={classnames(styles.createInsightSeriesButton,'button')}>

                            Done
                        </button>
                    </div>
                </FormGroup>

                <hr className={styles.createInsightSeparator}/>

                <FormGroup
                    name='Step between data points'
                    description='The distance between two data points on the chart'
                    className={styles.createInsightFormField}>

                    <div className={styles.createInsightRadioGroupContent}>

                        <input
                            type="text"
                            placeholder='ex. 2'
                            className={classnames(styles.createInsightStepInput, 'form-control')}
                        />

                        <FormRadioInput
                            name='Hours'
                            className={styles.createInsightRadio}/>
                        <FormRadioInput
                            name='Days'
                            className={styles.createInsightRadio}/>
                        <FormRadioInput
                            name='Weeks'
                            className={styles.createInsightRadio}/>
                        <FormRadioInput
                            name='Months'
                            className={styles.createInsightRadio}/>
                        <FormRadioInput
                            name='Years'
                            className={styles.createInsightRadio}/>
                    </div>
                </FormGroup>

                <hr className={styles.createInsightSeparator}/>

                <div>
                    <button
                        type='button'
                        className={classnames(styles.createInsightButton, styles.createInsightButtonActive, 'button')}>

                        Create code insight
                    </button>
                    <button
                        type='button'
                        className={classnames(styles.createInsightButton, 'button')}>

                        Cancel
                    </button>
                </div>
            </Form>
        </Page>
    )
