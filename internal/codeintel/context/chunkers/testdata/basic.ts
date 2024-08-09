// A module line comment

/**
 * A module block comment
 */

import * as vscode from 'vscode'
import { ExplainCodeAction } from '../code-actions/explain'

/**
 * Docblock for ExportedClass
 */
export class ExportedClass implements ExportedInterface {
    exportedInterfaceStringField: string

    /**
     * doc for exportedClassPrivateField
     */
    private exportedClassPrivateField: string

    // doc for exportedClassPublicMethod
    public exportedClassPublicMethod(): void {
        this.exportedClassPrivateField = 'hi'
    }

    private exportedClassPrivateMethod(): void {
    }

    exportedClassMethod(): void {
    }
}

class UnexportedClass {
    constructor(private unexportedClassPrivateField: number = 2) {
    }
}

export interface ExportedInterface {
    exportedInterfaceStringField: string
}

// Comment for UnexportedInterface
interface UnexportedInterface {
    unexportedInterfaceNumberFiled: number
}

/**
 * Doc for exportedFunc
 */
export function exportedFunc(): void {
}

function unexportedFunc(): void {
}

const variableFunc = (): void => {
}

// Doc for exportedLexicalFunc
export let exportedLexicalFunc = (): void => {
}

// Doc for exportedVar
export var exportedVar: number = 1;

let lexicalVar = 1;

const constVar = 'hello';
