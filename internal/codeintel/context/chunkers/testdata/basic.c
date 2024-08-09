/*
 * This is a comment
 */

#include <iostream.h>
#include "local.h"

#define MACRO_VAR

#ifdef MACRO_VAR
static struct foo_t *static_var;
function void func_inside_ifdef() {}
#endif

// struct foo_t docstring
struct foo_t {
	int foo_field1;
};

struct foo2_t {
};

struct bar_t {
	char bar_field1[50];
} var_struct;

// var_int docstring
int var_int;

// function do docstring
struct foo_t *do(int namesize)
{
	struct foo_t *ret;
	return ret;
}

int **double_pointer_function() {
	return null;
}

int main() {
	printf("hello world");
}
