/*
 * This is a package doc
 */
package org.apache.maven.api.settings;

import java.util.Comparator;

/**
 *  This is a javadoc
 **/
public class Application {
    public static void main(String[] args) {
        System.out.println("Hello, World");
    }
}

// Inline comment for FooClass
public class FooClass {
	// Inline comment for field bar
	private int barField;

	/**
	 * Constructor for Foo
	 *
	 * Multiline comment
	 */
	public FooClass(int bar) {
		String baz = bar;
		this.barField = baz;
		this.fooMethod();
	}

	// Non-block comment for fooMethod
	public int fooMethod() {
		int n = 1;
		System.out.println("doing foo");
		return n;
	}

	class InnerFoo {
		private int innerFooField = 123;

		public static void innerFooStaticMethod() {
		}
	}

	private static final Comparator ANON_COMPARATOR = new Comparator() {
		public int compare(Object aObj1, Object aObj2) {
			return -1;
		}
	};
}

/**
 * Block comment for FooInterface
 */
public interface FooInterface extends Serializable {
	FooClass getFoo();
}
