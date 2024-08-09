/**
 * A block style comment
 */

// A line comment

import foo from 'foo.js'

/**
 * Docblock for ExportedClass
 */
export class ExportedClass extends ParentClass {
	/**
	 * Doc for exportedClassField
	 */
	exportedClassField = 1;
	static staticExportedField;

	// doc for exportedClassPublicMethod
	exportedClassMethod() {
		this.exportedClassField = 'hi';
	}
}

class UnexportedClass {
	constructor() {
	}
}

/**
 * Doc for exportedFunc
 */
export function exportedFunc() {
}

function unexportedFunc() {
}

// Doc for exportedLexicalFunc
export let exportedLexicalFunc = () => {
}

// Doc for exportedLexicalConstFunc
export let exportedLexicalConstFunc = () => {
}

// Doc for exportedVar
export var exportedVar = 1;

/**
 * Doc for lexicalVar
 */
let lexicalVar = 1;

const constVar = 'hello';

export default function exportDefaultFunction() {}
export default class ExportDefaultClass {}
