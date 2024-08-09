import random
import asyncio

class MyClass:
    def method(self):
        return "Hello from a method!"

def foo():
    """This is a docstring"""
    x = 5
    y = 10
    if x > y:
        print("x is greater than y")
    else:
        print("y is greater than x")
    for i in range(5):
        print(i)
    while True:
        response = input("Continue? (y/n)")
        if response == 'n':
            break
    try:
        num = int(input("Enter a number: "))
    except ValueError:
        print("That was not a number!")
    instance = MyClass()
    print(instance.method())
print(random.randint(1, 100))
[foo() for _ in range(3)]  # Call foo 3 times using a list comprehension

def decorator(func):
    def wrapper():
        print("Something is happening before the function is called.")
        func()
        print("Something is happening after the function is called.")
    return wrapper

@decorator
def foo_decorated():
    print(" Foo is executed!")
foo_decorated()

def __init__(self, name):
    self.name = name


# Inheritance
class Animal:
    def __init__(self, name):
        self.name = name
    def eat(self):
        print(f"{self.name} is eating.")
class Dog(Animal):
    def bark(self):
        print(f"{self.name} says woof!")
dog = Dog("Rover")
dog.eat()  # Rover is eating.
dog.bark()  # Rover says woof!

# For else
def for_else():
   for num in range(10):
        if num == 5:
            continue  # Else block will be triggered
    else:
        print("Loop completed normally")

async def async_await():
    print("Hello")
    await asyncio.sleep(1)  # Pauses for 1 second
    print("World")
asyncio.run(async_await())


# structural pattern matching
match = (1, 2, 3)
a, b, c = match  # Equivalent to the current tuple unpacking
match = {'foo': 42, 'bar': 3.14}
{k: v for k, v in match if k == 'foo'}  # Using a expression + pattern match


def as_keyword():
    with open('file.txt') as f:
        text = f.read()
    x = 5
    y = x as z  # y is now also an alias for 5, as z
    try:
        raise ValueError('foo')
    except ValueError as err:
        print(err)  # Prints 'foo'
    match = {'foo': 42}
    {k as bar: v as baz for k, v in match}  # {'bar': 42}

# Generators:
def count_to_ten():
    for i in range(1, 11):
        yield i
counter = count_to_ten()
for num in counter:
    print(num)  # Prints 1-10

# Properties:
class Person:
    def __init__(self, first, last):
        self.firstname = first
        self.lastname = last
    @property
    def name(self):
        return f'{self.firstname} {self.lastname}'

p = Person('John', 'Doe')
p.name  # Calls the getter
# Magic methods:
class Adder:
    def __init__(self, n):
        self.n = n
    def __add__(self, other):
        return self.n + other.n
a = Adder(10)
b = Adder(5)
a + b  # 15
