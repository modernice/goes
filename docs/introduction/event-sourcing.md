<style lang="postcss" module>
.placeholder {
	@apply p-4 bg-zinc-900 rounded-lg text-zinc-400 text-sm;
}
</style>

# Event-Sourcing

In modern software design, especially for systems that require resilience,
scalability, and the capacity to evolve, selecting the right architecture is
essential. Event-sourcing is an architectural pattern that captures the entire
history of state changes through events, instead of just the latest state.
This approach can offer significant advantages for your applications.

## Benefits of Event-Sourcing

### Immutable Event Log

Event-sourcing enforces an immutable log of events which provides a reliable
audit trail and a means to replay and reconstruct past states of an application.
This can be essential for debugging, compliance, and audit requirements.

### Temporal Query Capability

Storing every state change as a unique event allows you to reconstruct the
application state at any point in time. This temporal aspect of event-sourcing
facilitates complex queries against the historical data, which is invaluable for
analytics and reporting.

### Decoupling and Scalability

The pattern inherently promotes a decoupled system architecture. Events are the
primary carriers of state changes, enabling a publish/subscribe mechanism that
allows components to operate independently and scale as needed.

### Adaptability to Change

Event-sourcing's approach to handling data evolution allows new event handlers
to be introduced without modifying existing events. This means the system is
more adaptable to change over time, making it easier to extend and modify as new
business requirements emerge.

### Integration with CQRS

Event-sourcing pairs well with the CQRS pattern, enabling the separation of
concerns between the read model (queries) and the write model (commands).
This can lead to more performant systems and cleaner, more maintainable code.

## When Not to Use Event-Sourcing

While event-sourcing provides numerous advantages, it's not always the best fit
for every project. Recognizing when not to use this pattern is just as important
as understanding its benefits. Here are some scenarios where event-sourcing
might not be the ideal choice:

### Simple Applications with Limited Business Complexity

If your application is straightforward, with limited business logic and without
complex domain interactions, the overhead of event-sourcing could introduce
unnecessary complexity. Traditional CRUD (Create, Read, Update, Delete)
approaches might be more appropriate and less costly to implement and maintain.

### Short-Lived Applications or Prototypes

For applications that are intended to be short-lived or are in the early
prototype stages, the initial setup and design required for event-sourcing
may not justify its benefits. Rapid prototyping often benefits from simpler
architectures that can be developed and iterated quickly.

### Teams New to Event-Sourcing

Event-sourcing can be challenging to implement correctly, especially for teams
that are not familiar with the pattern. The learning curve and the potential for
initial mistakes can lead to costly refactoring and rework. It's important that
teams have a solid understanding of the pattern and its implications before
adopting it.

### Use Cases Requiring Immediate Consistency

Event-sourcing naturally lends itself to eventual consistency, which may not be
suitable for systems that require immediate, transactional consistency.
In scenarios where each transaction must be immediately consistent, traditional
transactional systems might be a better fit.

### Lack of Tooling or Infrastructure Support

If the existing tooling, infrastructure, or organizational support for
event-sourcing is lacking, it may lead to difficulties in implementation and
operation. In such environments, other architectural patterns that are better
supported might lead to more successful outcomes.

## Learning Resources

Below is a list of reputable resources that cover various aspects of
event-sourcing, from foundational concepts to advanced techniques:

- **Greg Young: CQRS and Event Sourcing - Code on the Beach 2014**  
  [https://www.youtube.com/watch?v=JHGkaShoyNs](https://www.youtube.com/watch?v=JHGkaShoyNs)  
	A great talk on event-sourcing by Greg Young, the inventor of the pattern.

- **Greg Young: Event Sourcing: The bad parts - CodeCrafts 2022**  
  [https://www.youtube.com/watch?v=K4bj31fJGFk](https://www.youtube.com/watch?v=K4bj31fJGFk)  

- **Martin Fowler's Introduction to Event Sourcing**  
  [https://martinfowler.com/eaaDev/EventSourcing.html](https://martinfowler.com/eaaDev/EventSourcing.html)  
  An essential read for understanding the basics of event-sourcing from one of
	the thought leaders in software development.

- **Martin Fowler on CQRS**  
	[https://martinfowler.com/bliki/CQRS.html](https://martinfowler.com/bliki/CQRS.html)
	This article by Martin Fowler provides a great overview of the CQRS pattern,
	which is closely related to event-sourcing.

- **Martin Fowler: What do you mean by Event-Driven?**  
  [https://martinfowler.com/articles/201701-event-driven.html](https://martinfowler.com/articles/201701-event-driven.html)  

- **Microsoft's Guide to Event Sourcing**  
  [https://docs.microsoft.com/en-us/azure/architecture/patterns/event-sourcing](https://docs.microsoft.com/en-us/azure/architecture/patterns/event-sourcing)  
  This guide from Microsoft provides a thorough overview of the event-sourcing
  pattern, including scenarios and challenges.

- **Event Store's Event Sourcing Basics**  
  [https://www.eventstore.com/event-sourcing](https://www.eventstore.com/event-sourcing)  
  Event Store offers insights into the practical side of event-sourcing,
  showcasing how it works and why it's beneficial.

These resources provide a blend of theoretical understanding and practical
guidance, perfect for those who are looking to get a comprehensive view of
event-sourcing.
